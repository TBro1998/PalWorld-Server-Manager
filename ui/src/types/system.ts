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
