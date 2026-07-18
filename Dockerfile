# syntax=docker/dockerfile:1

# =============================================================================
# Stage 1: build the Next.js frontend (static export -> ui/out)
# =============================================================================
FROM oven/bun:1 AS frontend
WORKDIR /app/ui

# Install deps first for better layer caching.
COPY ui/package.json ui/bun.lock ./
RUN bun install --frozen-lockfile

# Build the static export. next.config.ts sets output:'export' distDir:'out'.
COPY ui/ ./
RUN bun run build

# =============================================================================
# Stage 2: build the Go binary with the frontend embedded (//go:embed ui/out)
# =============================================================================
FROM golang:1.26-bookworm AS backend

# Build-time version info injected by CI via --build-arg (docker build-push-action).
ARG VERSION=dev
ARG BUILD_TIME=unknown
ARG GIT_COMMIT=unknown

WORKDIR /app

# Module downloads cached on go.mod/go.sum.
COPY go.mod go.sum ./
RUN go mod download

# Source + the frontend build output required by the embed directive.
COPY . .
COPY --from=frontend /app/ui/out ./ui/out

# Pure-Go SQLite (modernc.org/sqlite) -> CGO not needed -> static binary.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
    -ldflags "-s -w \
      -X main.Version=${VERSION} \
      -X main.BuildTime=${BUILD_TIME} \
      -X main.GitCommit=${GIT_COMMIT}" \
    -o /out/psm .

# =============================================================================
# Stage 3: runtime image (glibc; Palworld/SteamCMD need it, not musl/Alpine)
# =============================================================================
FROM debian:bookworm-slim AS runtime

# SteamCMD is 32-bit and the Palworld Linux server needs C/C++ runtimes.
RUN dpkg --add-architecture i386 \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        lib32gcc-s1 \
        libstdc++6 \
        libstdc++6:i386 \
        locales \
        tzdata \
    && rm -rf /var/lib/apt/lists/*

# Non-root user: SteamCMD warns/misbehaves under root, and it is safer anyway.
RUN useradd --create-home --uid 10000 steam

COPY --from=backend /out/psm /usr/local/bin/psm
COPY --chmod=0755 docker/entrypoint.sh /usr/local/bin/entrypoint.sh

# Runtime configuration (no config.yaml -> the app reads these env vars).
# Paths live under /data so a single volume persists db/steamcmd/palworld/logs.
# JWT_SECRET is intentionally NOT set here: the app already defaults it
# (config.go envDefault), and baking a secret into an image layer is a smell.
# Override it via docker-compose / `docker run -e JWT_SECRET=...` in production.
ENV HOST=0.0.0.0 \
    PORT=8080 \
    DATABASE_PATH=/data/palworld.db \
    STEAMCMD_PATH=/data/steamcmd \
    LOG_DIR=/data/logs

USER steam
WORKDIR /data
VOLUME ["/data"]

# Web UI (tcp) + default Palworld game/query ports (udp). Adjust if you change
# the server's -port / -QueryPort launch args.
EXPOSE 8080/tcp 8211/udp 27015/udp

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
CMD ["psm"]
