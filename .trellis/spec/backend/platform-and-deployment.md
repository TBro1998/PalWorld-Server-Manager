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
