# Palworld Server Manager - Backend

> English | [中文](./README.md) | [日本語](./README.ja.md)

Go backend server for managing Palworld dedicated servers.

## Tech Stack

- **Framework**: Gin
- **Database**: SQLite (modernc.org/sqlite - pure Go, no CGO)
- **Authentication**: JWT
- **Real-time logs**: Server-Sent Events (SSE)

## Directory Structure

```
server/
├── main.go         # Application entry point
├── internal/
│   ├── api/           # HTTP handlers and routing
│   ├── auth/          # Authentication logic
│   ├── config/        # Configuration management
│   ├── database/      # Database initialization and migrations
│   ├── models/        # Data models
│   ├── server/        # HTTP server setup
│   ├── steamcmd/      # SteamCMD integration
│   └── i18n/          # Internationalization
└── pkg/
    ├── logger/        # Logging utilities
    └── utils/         # Common utilities
```

## Building

```bash
go mod download
go build .
```

## Running

```bash
./bin/palworld-server-manager
```

## Environment Variables

- `HOST` - Server host (default: 127.0.0.1)
- `PORT` - Server port (default: 8080)
- `DATABASE_PATH` - SQLite database path (default: ./data/palworld.db)
- `JWT_SECRET` - JWT signing secret (change in production!)
- `STEAMCMD_PATH` - SteamCMD installation path
- `PALWORLD_BASE_PATH` - Palworld servers base directory

## Embedding Frontend

The frontend is embedded using Go's `embed` package. Build the Next.js frontend first:

```bash
cd ../ui
bun run build
```

Then build the Go binary - it will include the frontend automatically.
