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
    case "node_modules":
      return "node_modules of projects you haven't touched";
    default:
      return "";
  }
}
