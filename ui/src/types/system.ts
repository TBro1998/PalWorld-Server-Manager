// Version and update types for the system update feature.

export interface VersionInfo {
  version: string
  buildTime: string
  gitCommit: string
}

export interface CheckResult {
  currentVersion: string
  isDev: boolean
  hasUpdate: boolean
  latestVersion?: string
  releaseNotes?: string
  assetName?: string
  assetURL?: string
  assetSize?: number
  checkedAt?: string
  err?: string
}

export interface SystemSettings {
  download_mirror: string
}

// Whole-host resource usage. Mirrors sysstat.HostStats on the backend.
// Returned by GET /api/system/stats. cpuPercent is normalized across cores
// (0..100); memory/disk values are bytes.
export interface HostStats {
  cpuPercent: number
  numCpu: number
  memUsed: number
  memTotal: number
  memPercent: number
  diskUsed: number
  diskTotal: number
  diskPercent: number
}

// Per-server process-tree resource usage. Mirrors sysstat.ProcessStats.
// Returned by GET /api/servers/:id/stats. cpuPercent is per-core (may exceed
// 100). When the server is not running, running=false and reason explains why.
export interface ProcessStats {
  running: boolean
  reason?: string
  pid?: number
  cpuPercent: number
  numCpu: number
  memoryRss: number
  processCount: number
}

export interface UpdateProgressEvent {
  pct: number
  msg: string
}

// Mirrors update.UpdateStatus on the backend.  Returned by
// GET /api/system/update/status so the UI can recover in-progress state
// after a page navigation or browser refresh.
export interface UpdateStatus {
  phase: 'idle' | 'downloading' | 'restarting' | 'error'
  pct: number
  msg: string
  err?: string
}
