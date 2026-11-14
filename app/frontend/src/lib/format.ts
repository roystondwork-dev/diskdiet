// Sizes use decimal units (1 GB = 1,000,000,000 bytes), like Finder, Explorer,
// Docker and the diskdiet CLI.
const UNITS = ["B", "kB", "MB", "GB", "TB", "PB"];

export function formatBytes(n: number): string {
  if (!n || n < 0) return "0 B";
  if (n < 1000) return `${n} B`;
  let v = n;
  let i = 0;
  while (v >= 999.95 && i < UNITS.length - 1) {
    v /= 1000;
    i++;
  }
  return `${v.toFixed(1)} ${UNITS[i]}`;
}

export function idle(days: number): string {
  if (days >= 365) return `${(days / 365).toFixed(1)} years`;
  if (days >= 60) return `${Math.floor(days / 30)} months`;
  if (days === 1) return "1 day";
  if (days === 0) return "today";
  return `${days} days`;
}

export function percent(part: number, whole: number): string {
  if (!whole) return "0%";
  const p = (part / whole) * 100;
  return p > 0 && p < 1 ? "<1%" : `${Math.round(p)}%`;
}

// Sources ticked by default: removing their old entries costs at most a
// re-download. Project folders and Docker wait until the user ticks them.
export function safeByDefault(id: string): boolean {
  return !isProjects(id) && id !== "docker";
}

// Sources whose removable items are project folders the user can untick.
export function isProjects(id: string): boolean {
  return id === "node_modules" || id === "target";
}

// What a clean removes from each source, in plain words.
export function describe(id: string, volumes: boolean): string {
  switch (id) {
    case "npm":
      return "Cached packages and metadata, old npx installs and logs";
    case "yarn":
    case "bun":
      return "Cached packages";
    case "pnpm":
      return "Packages no project uses (pnpm store prune) and old metadata";
    case "docker":
      return volumes
        ? "Unused images, stopped containers, build cache and unused volumes"
        : "Unused images, stopped containers and build cache";
    case "pip":
      return "Cached downloads and built wheels";
    case "uv":
      return "The whole uv cache (uv cache clean)";
    case "cargo":
      return "Downloaded crates, unpacked sources and git checkouts";
    case "go":
      return "Build cache entries and module versions";
    case "gradle":
      return "Downloaded Gradle versions and per-version caches";
    case "maven":
      return "Artifact versions in ~/.m2";
    case "playwright":
      return "Older browser versions; the newest of each stays";
    case "cypress":
      return "Older Cypress versions; the newest stays";
    case "xcode":
      return "DerivedData and device support files (never Archives)";
    case "simulators":
      return "Simulators for iOS versions no longer installed, and old simulator caches";
    case "cocoapods":
      return "Old pod versions in the download cache (never a project's Pods)";
    case "swiftpm":
      return "Cached Swift package repositories and artifacts";
    case "android":
      return "System images no emulator uses, older NDKs and build tools; the newest stay";
    case "homebrew":
      return "Old downloads in Homebrew's cache; installed software stays";
    case "node_modules":
      return "node_modules of projects you haven't touched";
    case "target":
      return "target folders of Rust and Maven projects you haven't touched";
    default:
      return "";
  }
}
