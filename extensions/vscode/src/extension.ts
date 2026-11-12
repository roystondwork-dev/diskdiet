// diskdiet for VS Code: a "Disk space" view, a status bar item, background
// checks, and a fix offer when a command in the terminal runs out of disk.
// All disk work is done by the diskdiet CLI (see cli.ts).

import * as fs from "node:fs";
import * as os from "node:os";
import * as vscode from "vscode";
import { DiskdietClient, resolveBinary } from "./cli";
import {
  ageWords,
  cleanable,
  DiskFullScanner,
  foundSources,
  formatBytes,
  freeChoices,
  isProjects,
  missingSources,
  plannedBytes,
  projectDescription,
  projectLabel,
  sourceDescription,
  sourceTooltip,
  status,
  type StatusMode,
  summarizeClean,
  tildify,
} from "./model";
import { nodeExec } from "./nodeExec";
import type { Project, ScanData, SourceReport } from "./schema";

/** A scan older than this is redone before cleaning. */
const FRESH_MS = 10 * 60_000;

type Node =
  | { kind: "source"; s: SourceReport }
  | { kind: "location"; path: string }
  | { kind: "project"; p: Project; sourceId: string }
  | { kind: "note"; text: string }
  | { kind: "message"; text: string };

interface Settings {
  path: string;
  olderThanDays: number;
  statusBar: StatusMode;
  warnAboveGB: number;
  checkEveryHours: number;
  detectDiskFull: boolean;
}

function settings(): Settings {
  const c = vscode.workspace.getConfiguration("diskdiet");
  return {
    path: c.get<string>("path", ""),
    olderThanDays: c.get<number>("olderThanDays", 30),
    statusBar: c.get<StatusMode>("statusBar", "aboveThreshold"),
    warnAboveGB: c.get<number>("warnAboveGB", 10),
    checkEveryHours: c.get<number>("checkEveryHours", 6),
    detectDiskFull: c.get<boolean>("detectDiskFull", true),
  };
}

class Diskdiet implements vscode.TreeDataProvider<Node>, vscode.Disposable {
  private readonly changed = new vscode.EventEmitter<Node | undefined>();
  readonly onDidChangeTreeData = this.changed.event;
  private readonly disposables: vscode.Disposable[] = [];
  private readonly statusItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 50);
  private readonly home = os.homedir();
  private scan: ScanData | undefined;
  private scannedAt = 0;
  private scanning: Promise<ScanData | undefined> | undefined;
  private error = "";
  private timer: NodeJS.Timeout | undefined;
  private warned = false;

  constructor(
    private readonly ctx: vscode.ExtensionContext,
    private readonly log: vscode.LogOutputChannel,
  ) {
    this.statusItem.command = "diskdiet.showView";
    this.disposables.push(
      this.statusItem,
      this.changed,
      vscode.window.registerTreeDataProvider("diskdiet.sources", this),
      vscode.commands.registerCommand("diskdiet.refresh", () => this.refresh()),
      vscode.commands.registerCommand("diskdiet.free", () => this.free()),
      vscode.commands.registerCommand("diskdiet.freeSource", (n?: Node) => this.free(n?.kind === "source" ? [n.s.id] : undefined)),
      vscode.commands.registerCommand("diskdiet.reveal", (n?: Node) => this.reveal(n)),
      vscode.commands.registerCommand("diskdiet.setAge", () => this.setAge()),
      vscode.commands.registerCommand("diskdiet.showView", () => vscode.commands.executeCommand("workbench.view.extension.diskdiet")),
      vscode.workspace.onDidChangeConfiguration((e) => {
        if (e.affectsConfiguration("diskdiet.olderThanDays") || e.affectsConfiguration("diskdiet.path")) {
          void this.refresh();
        } else if (e.affectsConfiguration("diskdiet")) {
          this.showStatus();
          this.schedule();
        }
      }),
      vscode.window.onDidStartTerminalShellExecution((e) => void this.watchTerminal(e.execution)),
    );
    this.showStatus();
    this.schedule(15_000); // first check soon after start, without slowing it down
  }

  private client(): DiskdietClient {
    const binary = resolveBinary({
      configured: settings().path,
      extensionPath: this.ctx.extensionPath,
      platform: process.platform,
      arch: process.arch,
      exists: (f) => fs.existsSync(f),
    });
    return new DiskdietClient(binary, nodeExec);
  }

  private schedule(firstMs?: number): void {
    if (this.timer) {
      clearTimeout(this.timer);
    }
    const hours = Math.max(settings().checkEveryHours, 0);
    if (firstMs === undefined && hours === 0) {
      return;
    }
    this.timer = setTimeout(
      () => {
        void this.refresh(true).then(() => {
          this.schedule();
        });
      },
      firstMs ?? hours * 3_600_000,
    );
  }

  /** Scans again. Background scans may notify when a lot can be freed. */
  async refresh(background = false): Promise<ScanData | undefined> {
    if (this.scanning) {
      return this.scanning;
    }
    const run = async (): Promise<ScanData | undefined> => {
      const s = settings();
      try {
        this.scan = await this.client().scan(s.olderThanDays);
        this.scannedAt = Date.now();
        this.error = "";
        this.log.info(`scan: ${formatBytes(this.scan.prunableBytes)} of ${formatBytes(this.scan.bytes)} can be freed`);
      } catch (err) {
        this.error = err instanceof Error ? err.message : String(err);
        this.log.error(`scan: ${this.error}`);
        if (!background) {
          void vscode.window.showErrorMessage(`diskdiet: ${this.error}`);
        }
      }
      this.changed.fire(undefined);
      this.showStatus();
      if (background) {
        this.maybeWarn();
      }
      return this.scan;
    };
    this.scanning = background
      ? run()
      : Promise.resolve(vscode.window.withProgress({ location: { viewId: "diskdiet.sources" } }, run));
    try {
      return await this.scanning;
    } finally {
      this.scanning = undefined;
    }
  }

  private showStatus(): void {
    const s = settings();
    const st = status(this.scan, s.statusBar, s.warnAboveGB, s.olderThanDays);
    this.statusItem.text = st.text;
    this.statusItem.tooltip = this.error ? `diskdiet: ${this.error}` : st.tooltip;
    this.statusItem.backgroundColor = st.warn ? new vscode.ThemeColor("statusBarItem.warningBackground") : undefined;
    if (st.show) {
      this.statusItem.show();
    } else {
      this.statusItem.hide();
    }
  }

  private maybeWarn(): void {
    const s = settings();
    if (this.warned || !this.scan || s.warnAboveGB <= 0 || this.scan.prunableBytes < s.warnAboveGB * 1e9) {
      return;
    }
    this.warned = true; // once per session
    void vscode.window
      .showInformationMessage(`diskdiet: ${formatBytes(this.scan.prunableBytes)} of developer caches ${ageWords(s.olderThanDays)} can be freed.`, "Free…", "Show")
      .then((pick) => {
        if (pick === "Free…") {
          void this.free();
        } else if (pick === "Show") {
          void vscode.commands.executeCommand("diskdiet.showView");
        }
      });
  }

  getTreeItem(n: Node): vscode.TreeItem {
    switch (n.kind) {
      case "source": {
        const hasChildren = (n.s.locations?.length ?? 0) > 0 || (n.s.projects?.length ?? 0) > 0 || (n.s.notes?.length ?? 0) > 0;
        const item = new vscode.TreeItem(n.s.name, hasChildren ? vscode.TreeItemCollapsibleState.Collapsed : vscode.TreeItemCollapsibleState.None);
        item.description = sourceDescription(n.s);
        item.tooltip = sourceTooltip(n.s, settings().olderThanDays, this.home);
        item.contextValue = cleanable(n.s) ? "source.cleanable" : "source";
        item.iconPath = new vscode.ThemeIcon(cleanable(n.s) ? "trash" : "database");
        return item;
      }
      case "location": {
        const item = new vscode.TreeItem(tildify(n.path, this.home));
        item.iconPath = vscode.ThemeIcon.Folder;
        item.contextValue = "path";
        item.tooltip = n.path;
        item.command = { command: "diskdiet.reveal", title: "Reveal", arguments: [n] };
        return item;
      }
      case "project": {
        const item = new vscode.TreeItem(projectLabel(n.p, this.home));
        item.description = projectDescription(n.p);
        item.tooltip = `${n.p.path}\nRemoved by a clean; reinstall or rebuild brings it back.`;
        item.iconPath = new vscode.ThemeIcon("folder-library");
        item.contextValue = "path";
        return item;
      }
      case "note": {
        const item = new vscode.TreeItem(n.text);
        item.iconPath = new vscode.ThemeIcon("info");
        item.tooltip = n.text;
        return item;
      }
      case "message":
        return new vscode.TreeItem(n.text);
    }
  }

  getChildren(n?: Node): Node[] {
    if (!n) {
      if (this.error && !this.scan) {
        return [{ kind: "message", text: `⚠ ${this.error}` }];
      }
      if (!this.scan) {
        return [];
      }
      const days = settings().olderThanDays;
      const head: Node = {
        kind: "message",
        text: `${formatBytes(this.scan.prunableBytes)} ${ageWords(days)} can be freed, of ${formatBytes(this.scan.bytes)}`,
      };
      const rows: Node[] = foundSources(this.scan).map((s) => ({ kind: "source", s }));
      const missing = missingSources(this.scan);
      const tail: Node[] = missing.length > 0 ? [{ kind: "note", text: `Not found: ${missing.join(", ")}` }] : [];
      return [head, ...rows, ...tail];
    }
    if (n.kind !== "source") {
      return [];
    }
    const out: Node[] = [];
    if (isProjects(n.s.id)) {
      for (const p of n.s.projects ?? []) {
        out.push({ kind: "project", p, sourceId: n.s.id });
      }
    } else {
      for (const l of n.s.locations ?? []) {
        if (l.startsWith("/") || /^[A-Za-z]:[\\/]/.test(l)) {
          out.push({ kind: "location", path: l });
        } else {
          out.push({ kind: "note", text: l });
        }
      }
    }
    for (const note of n.s.notes ?? []) {
      out.push({ kind: "note", text: note });
    }
    return out;
  }

  private async reveal(n?: Node): Promise<void> {
    const p = n?.kind === "location" ? n.path : n?.kind === "project" ? n.p.path : undefined;
    if (p) {
      await vscode.commands.executeCommand("revealFileInOS", vscode.Uri.file(p));
    }
  }

  private async setAge(): Promise<void> {
    const current = settings().olderThanDays;
    const picks = [7, 30, 90, 0].map((d) => ({
      label: d === 0 ? "Everything" : `Unused for ${d} days`,
      description: d === current ? "current" : "",
      days: d,
    }));
    const pick = await vscode.window.showQuickPick(picks, { title: "diskdiet: what counts as old?" });
    if (pick) {
      await vscode.workspace.getConfiguration("diskdiet").update("olderThanDays", pick.days, vscode.ConfigurationTarget.Global);
    }
  }

  /** Lets the user choose what to free, confirms, and runs diskdiet clean. */
  async free(preselect?: string[]): Promise<void> {
    let scan = this.scan;
    if (!scan || Date.now() - this.scannedAt > FRESH_MS) {
      scan = await this.refresh();
    }
    if (!scan) {
      return;
    }
    const days = settings().olderThanDays;
    const choices = freeChoices(scan).map((c) => ({ ...c, picked: preselect ? preselect.includes(c.id) : c.picked }));
    if (choices.length === 0) {
      void vscode.window.showInformationMessage(`diskdiet: nothing ${ageWords(days)} to free. "diskdiet: Set What Counts as Old" finds more.`);
      return;
    }
    const chosen = await vscode.window.showQuickPick(choices, {
      canPickMany: true,
      title: `diskdiet: free space (${ageWords(days)})`,
      placeHolder: "Choose what to free; node_modules, target folders and Docker are off unless you tick them",
    });
    if (!chosen || chosen.length === 0) {
      return;
    }
    const ids = chosen.map((c) => c.id);
    const keep: string[] = [];
    for (const s of scan.sources) {
      if (!ids.includes(s.id) || !isProjects(s.id) || !s.projects || s.projects.length === 0) {
        continue;
      }
      const picks = await vscode.window.showQuickPick(
        s.projects.map((p) => ({ label: projectLabel(p, this.home), description: projectDescription(p), picked: true, path: p.path })),
        { canPickMany: true, title: `diskdiet: which ${s.name} to remove? Untick any to keep` },
      );
      if (!picks) {
        return;
      }
      const remove = new Set(picks.map((p) => p.path));
      keep.push(...s.projects.filter((p) => !remove.has(p.path)).map((p) => p.path));
    }
    const planned = plannedBytes(scan, ids, keep);
    const names = chosen.map((c) => c.label).join(", ");
    const button = `Free ${formatBytes(planned)}`;
    const ok = await vscode.window.showWarningMessage(
      `Free about ${formatBytes(planned)} from ${names}?`,
      { modal: true, detail: "Removed cache entries download again when a project needs them; node_modules come back with an install, target folders with a build." },
      button,
    );
    if (ok !== button) {
      return;
    }
    try {
      const result = await vscode.window.withProgress(
        { location: vscode.ProgressLocation.Notification, title: `diskdiet: freeing space in ${names}…` },
        () => this.client().clean(ids, days, keep),
      );
      const sum = summarizeClean(result);
      for (const line of sum.details) {
        this.log.info(line);
      }
      const show = sum.problems ? vscode.window.showWarningMessage : vscode.window.showInformationMessage;
      void show(sum.message, "Show details").then((pick) => {
        if (pick) {
          this.log.show();
        }
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : String(err);
      this.log.error(`clean: ${msg}`);
      void vscode.window.showErrorMessage(`diskdiet: ${msg}`);
    }
    await this.refresh();
  }

  /** Offers to free space when a command in the terminal runs out of disk. */
  private async watchTerminal(execution: vscode.TerminalShellExecution): Promise<void> {
    if (!settings().detectDiskFull) {
      return;
    }
    const scanner = new DiskFullScanner();
    try {
      for await (const chunk of execution.read()) {
        if (scanner.push(chunk)) {
          void this.offerFree(execution.commandLine.value);
          return;
        }
      }
    } catch (err) {
      this.log.warn(`reading terminal output: ${String(err)}`);
    }
  }

  private async offerFree(commandLine: string): Promise<void> {
    this.log.info(`disk full, seen in terminal output of: ${commandLine}`);
    const scan = await this.refresh(true);
    const what = scan && scan.prunableBytes > 0 ? ` diskdiet can free up to ${formatBytes(scan.prunableBytes)}.` : "";
    const pick = await vscode.window.showWarningMessage(`The disk is full (no space left on device).${what}`, "Free Disk Space…", "Show");
    if (pick === "Free Disk Space…") {
      await this.free();
    } else if (pick === "Show") {
      await vscode.commands.executeCommand("diskdiet.showView");
    }
  }

  dispose(): void {
    if (this.timer) {
      clearTimeout(this.timer);
    }
    for (const d of this.disposables) {
      d.dispose();
    }
  }
}

export function activate(ctx: vscode.ExtensionContext): void {
  const log = vscode.window.createOutputChannel("diskdiet", { log: true });
  ctx.subscriptions.push(log, new Diskdiet(ctx, log));
}

export function deactivate(): void {
  // Everything is disposed through the extension context.
}
