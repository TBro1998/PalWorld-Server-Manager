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
