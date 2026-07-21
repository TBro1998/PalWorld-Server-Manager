# Platform & Deployment Guidelines

> Cross-platform (Windows/Linux) conventions and the Docker run mode.
> Source of truth: verified against the codebase on 2026-07-14.

---

## Cross-platform code layout

Platform differences are isolated with build tags or `runtime.GOOS`, never
`#ifdef`-style branching scattered through business logic. Keep this pattern:

| Concern | Windows | Linux/Unix | Where |
|---|---|---|---|
| Process group / kill / liveness | `platform_windows.go` (`CREATE_NEW_PROCESS_GROUP`, `taskkill`, `tasklist`) | `platform_unix.go` (`Setpgid`, `syscall.Kill`) | `internal/process/` |
| Server launcher name | `PalServer.exe` | `PalServer.sh` | `internal/process/platform.go` |
| SteamCMD executable | `steamcmd.exe` | `steamcmd.sh` | `internal/steamcmd/steamcmd.go:getExecutablePath` |
| SteamCMD download | `steamcmd.zip` (zip) | `steamcmd_linux.tar.gz` (tar.gz) | `internal/steamcmd/download.go` |
| Steam client library links | no-op | `steamclient_unix.go` | `internal/steamcmd/steamclient_*.go` |

**Rule**: when adding a platform-specific behavior, add a `_windows.go` /
`_unix.go` (or `!windows`) pair with the *same exported function signature* so
callers stay platform-agnostic. Example: `EnsureSteamClientLinks(steamcmdPath)`
does the real work on Unix and returns `nil` on Windows.

---

## Linux runtime requirement: Steam client library symlinks

The Palworld **Linux** dedicated server looks up `steamclient.so` under
`$HOME/.steam/sdk64` (and `sdk32`), but SteamCMD unpacks it into its own
`linux64/`/`linux32/` dirs. Without the symlinks the server exits early / spams
`steamclient.so missing`.

- `steamcmd.EnsureSteamClientLinks` creates `~/.steam/sdk64/steamclient.so →
  <steamcmd>/linux64/steamclient.so` (and the sdk32 pair). It is idempotent and
  **best-effort** (a failure is logged, never fatal — matches the tolerant tone
  of `runInitialUpdate`).
- Called from `steamcmd.CheckAndInstall` on **both** the "already installed" and
  "freshly installed" paths, so native Linux and Docker both get it.
- Symlinks may point at a not-yet-existing target; they resolve once SteamCMD
  finishes unpacking. Do not gate creation on the target existing.

---

## Server runtime log capture (stdout takeover)

The manager captures a running server's log by taking over the spawned process's
stdout/stderr (`cmd.Stdout = out; cmd.Stderr = out`, one `io.MultiWriter` →
disk capture + live SSE). Palworld writes **no log file by default** (no
`Pal/Saved/Logs/Pal.log`); everything goes to stdout and is discarded on exit —
so tailing a log file does not work and was removed.

Two platform-specific pieces make the takeover actually work (`launchTarget` /
`logArgs` in `platform_{windows,unix}.go`):

- **Windows**: `PalServer.exe` is only a launcher that starts the real server in
  a **separate console** we cannot capture. Spawn the console binary directly:
  `<install>/Pal/Binaries/Win64/PalServer-Win64-Shipping-Cmd.exe`, from its own
  `Win64` dir (that dir holds `steam_appid.txt` + required DLLs; Saved/Config
  still resolve from the exe-relative project root). And append UE flags
  `-log -stdout -FullStdOutLogOutput -UTF8Output` — **without `-stdout` UE writes
  only to its console window, never the redirected stdout handle**, so nothing is
  captured. `-UTF8Output` avoids UTF-16 mojibake. Verified: these produce live,
  per-line-flushed UE log lines on the captured pipe.
- **Linux**: `PalServer.sh` runs the binary in the same process tree; its stdout
  is inherited and captured directly, and the Linux build already emits to stdout
  by default, so `logArgs()` is empty.

`serverExecutable` (PalServer.exe / PalServer.sh) is still the install-presence
marker for `IsInstalled`; it is **not** the launch target — use `launchTarget`.

Adopted processes (reconciled by PID after a manager restart) cannot have their
stdout captured (the handle is fixed at spawn), so they get no live log — an
accepted limitation, not a regression.

---

## No browser auto-open in the product

The tool must **never** open a browser on startup — it is a headless server
process (Windows service, Linux daemon, Docker). Browser-open is a *debug-only*
convenience and lives in `debug.bat` (a delayed `start "" http://127.0.0.1:8080/`
before `go run .`). Do not reintroduce `openBrowser`/`xdg-open`/`rundll32` into
the server package.

---

## Docker run mode (manager + game server, same container)

Contract encoded in `Dockerfile` + `docker-compose.yml` + `docker/entrypoint.sh`:

- **Three-stage build**: `oven/bun` (frontend static export → `ui/out`) →
  `golang:1.26-bookworm` (Go build, embeds `ui/out`) → `debian:bookworm-slim`
  runtime.
- **glibc base, not Alpine/musl**: the Palworld x86_64 binary and 32-bit SteamCMD
  need glibc + `lib32gcc-s1` + `libstdc++6(:i386)`.
- **`CGO_ENABLED=0`**: SQLite is pure Go (`modernc.org/sqlite`), so the Linux
  binary is static and needs no C toolchain. Always cross-compile Linux with
  `CGO_ENABLED=0` (the default cgo-on path fails on a Windows host without gcc).
- **Non-root `steam` user**: SteamCMD misbehaves as root.
- **SteamCMD & game server are NOT baked into the image** — the app downloads
  them at first run into the `/data` volume. Keeps the image small (~150MB) and
  lets the game server update without rebuilding.
- **Config via env** (no `config.yaml` in the image → env branch of
  `config.Load`): `HOST=0.0.0.0`, paths under `/data`. **Do not** bake
  `JWT_SECRET` as an image `ENV` (Docker `SecretsUsedInArgOrEnv` warning); the
  app already defaults it via `envDefault`, and compose/`-e` overrides it.

### Build-context hygiene (`.dockerignore`)

The repo root accumulates **runtime data** during local dev (installed
`steamcmd/`, `Servers/`, `palworld/`, `temp/`, `psm-data/`). These can reach
**many GB** and will balloon the build context (observed 11GB → build failure)
if not excluded. Keep them in `.dockerignore`. Also exclude `ui/out` /
`ui/node_modules` (rebuilt in-image).

### Shell scripts must be LF

`docker/entrypoint.sh` and any `*.sh` must stay LF-only (`.gitattributes:
*.sh text eol=lf`). A CRLF shebang breaks execution inside Linux containers.

### Ports

`8080/tcp` (web UI), `8211/udp` (game), `27015/udp` (query). If a server's
`-port` / `-QueryPort` launch args change, the compose port mapping must change
to match.

---

## OS resource sampling (CPU / memory / disk) — `internal/sysstat`

Host and per-server-process resource metrics are sampled with
`github.com/shirou/gopsutil/v4` (`cpu`, `mem`, `disk`, `process`). gopsutil is
pure Go and works under `CGO_ENABLED=0` on both Windows and Linux, so **no
`platform_*.go` pair and no `tasklist`/`wmic`/`ps` shelling** is needed — its
`Children()`/`Times()`/`MemoryInfo()` already abstract the platform (Windows
toolhelp snapshot vs Linux `/proc`). Prefer this over hand-rolled platform code
for any new OS-metric need.

### Signatures (`sysstat.Collector`, held once by `api.Router`)

- `New() *Collector` — resolves `cpu.Counts(true)` once (falls back to 1).
- `Host(ctx) HostStats` — never returns an error; each of cpu/mem/disk is
  sampled independently and left at zero on failure (endpoint always 200).
- `Process(ctx, key string, pid int) ProcessStats` — aggregates the whole
  process tree rooted at pid.

### Contracts

- `HostStats{cpuPercent(0-100 norm), numCpu, memUsed/Total/Percent, diskUsed/Total/Percent}`.
  Disk is `disk.Usage(".")` — the volume holding the working dir (the data disk);
  in Docker that is the mounted `/data` volume, which is what we want.
- `ProcessStats{running, reason, pid, cpuPercent(per-core, may exceed 100), numCpu, memoryRss, processCount}`.
  Not-running → `{running:false, reason:"not_running", numCpu}` (structured 200,
  mirrors the palapi rest-handler degradation style), NOT a 500.

### Process-tree aggregation is mandatory on Windows

Same root cause as the log-capture note above: `PalServer.exe` is only a
launcher; the real server is a child (`PalServer-Win64-Shipping-Cmd.exe`).
Sampling only the recorded PID reports near-zero CPU/RSS. `gatherTree` recurses
`ChildrenWithContext` from the root, summing `MemoryInfo().RSS` and
`Times().User+System`, with:
- a `seen map[int32]struct{}` cycle/re-visit guard (never assume the tree is acyclic),
- per-node error skip (a child that just exited must not fail the whole sample),
- `count==0` after the walk ⇒ treat as not-running.

The manager's PID comes from a read-only `Manager.PID(serverID)` getter
(running-handle pid, DB `Server.PID` fallback) — it does not mutate lifecycle state.

### CPU-delta baseline pattern

gopsutil's `Process.Percent(0)` caches its baseline *inside the Process struct*,
which is useless when each poll builds a fresh tree of Process objects. So the
Collector keeps its own `map[key]cpuSample{totalCPUSeconds, at}` under a
`sync.Mutex` and computes `deltaCPUSeconds / deltaWallSeconds * 100` itself:
- store the new baseline on **every** call, before any early return;
- first frame OR baseline older than `baselineExpiry` (30s) ⇒ return 0 (next 5s poll corrects);
- guard `wall <= 0` (two rapid calls) and clamp negative deltas to 0;
- process metric is **per-core** (no `/numCPU`); host `cpu.Percent(0,false)` is already normalized.

> **Warning**: `go test -race` requires CGO (a C compiler). The Windows dev host
> has no gcc, so `-race` cannot run there — the sysstat mutex coverage is verified
> by inspection and plain `go test` instead. Don't add a `-race` step to a
> Windows-only verification flow expecting it to pass.
