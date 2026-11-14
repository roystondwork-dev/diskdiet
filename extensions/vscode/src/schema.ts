// The diskdiet CLI's JSON output (schema version 1): `diskdiet scan --json`
// and `diskdiet clean --json`.

export const SCHEMA_VERSION = 1;

export interface Project {
  path: string;
  bytes: number;
  idleDays: number;
}

export interface SourceReport {
  id: string;
  name: string;
  found: boolean;
  locations?: string[];
  bytes: number;
  items: number;
  prunableBytes: number;
  prunableItems: number;
  estimate?: boolean;
  unavailable?: boolean;
  notes?: string[];
  error?: string;
  projects?: Project[];
}

export interface ScanData {
  schemaVersion: number;
  olderThan: string;
  sources: SourceReport[];
  bytes: number;
  prunableBytes: number;
}

export interface SourceResult {
  id: string;
  name: string;
  freedBytes: number;
  trashedBytes?: number;
  removed: number;
  skipped?: string;
  notes?: string[];
  errors?: string[];
}

export interface CleanData {
  schemaVersion: number;
  olderThan: string;
  dryRun: boolean;
  plan: SourceReport[];
  results: SourceResult[];
  freedBytes: number;
  trashedBytes?: number;
}
