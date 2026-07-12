# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

PalWorld Server Manager (幻兽帕鲁服务器开服工具) is a web-based management tool for Palworld dedicated servers. It provides a single-binary application that embeds both the Go backend and Next.js frontend, supporting features like server lifecycle management, mod installation via SteamCMD, real-time monitoring, and multi-language support (Chinese, English, Japanese).

**Key Architecture Pattern**: The frontend is built as a static export and embedded into the Go binary using `embed.FS`, creating a self-contained executable (~15-25MB) that serves both the web UI and REST API.

## Build Commands

### Complete Build (Frontend + Backend)

```bash
# 1. Build frontend first
cd ui
bun install
bun run build  # Creates static export in ui/out/

# 2. Build backend from project root (embeds frontend automatically)
cd ..
go mod download
go build .
```

### Development Workflow

**Frontend Development:**
```bash
cd ui
bun run dev  # Development server on http://localhost:3000
bun run lint # ESLint
```

**Backend Development:**
```bash
# Run from project root
go run .  # Run without building
go build .
```

**Note**: During backend development, you must rebuild the frontend if you make UI changes, as the backend embeds the static build.

## Configuration System

The application uses a **three-tier configuration priority**:
1. `config.yaml` file (highest priority)
2. Environment variables
3. Hardcoded defaults (lowest priority)

Configuration is loaded in `internal/config/config.go`. If `config.yaml` exists, it takes precedence over environment variables entirely.

**Key Configuration Fields:**
- `host` / `HOST` - Web interface listen address (default: 127.0.0.1)
- `port` / `PORT` - Web interface port (default: 8080)
- `jwt_secret` / `JWT_SECRET` - JWT signing key (must change in production)
- `github_repo` / `GITHUB_REPO` - Update source repository (default: TBro1998/PalWorld-Server-Manager)
- `steamcmd_path` / `STEAMCMD_PATH` - SteamCMD installation path
- `palworld_base_path` / `PALWORLD_BASE_PATH` - Palworld server base directory

**Auto-update is always enabled**; updates are performed via the web UI.

## Architecture

### Backend Architecture (Go + Gin)

**Entry Point:** `main.go` (project root)
- Loads configuration (config.yaml or env vars)
- Initializes SQLite database with migrations
- Embeds frontend static files via `//go:embed all:ui/out`
- Starts HTTP server

**Directory Structure:**
```
/ (project root)
├── main.go             # Main entry point
├── internal/
│   ├── api/            # HTTP handlers, routing (REST API endpoints)
│   ├── config/         # Configuration loading (YAML + env priority)
│   ├── database/       # SQLite initialization & migrations
│   ├── models/         # Data models (Server, Mod, User)
│   ├── server/         # HTTP server setup (Gin, static file serving)
│   ├── auth/           # JWT authentication (to be implemented)
│   ├── steamcmd/       # SteamCMD integration (to be implemented)
│   └── i18n/           # Backend i18n (to be implemented)
├── pkg/                # Public packages (logger, utils - to be implemented)
└── ui/                 # Frontend (Next.js)
```

**Key Components:**
- **Database**: SQLite with pure Go driver (`modernc.org/sqlite`), no CGO required
- **HTTP Framework**: Gin for routing and middleware
- **Static Serving**: Embedded Next.js build served via `http.FS` with fallback routing
- **Real-time Logs**: Server-Sent Events (SSE) for streaming logs (to be implemented)

**API Structure** (`internal/api/router.go`):
- `/api/auth/*` - Authentication (login, register)
- `/api/servers/*` - Server management, start/stop/restart, logs
- `/api/servers/:serverId/mods/*` - Mod management
- `/api/system/stats` - System monitoring

### Frontend Architecture (Next.js 16 App Router)

**Build Output:** Static export to `ui/out/` which gets embedded into Go binary

**Directory Structure:**
```
ui/
├── src/
│   ├── app/            # App Router pages (direct routing, no locale prefix)
│   ├── components/     # React components
│   ├── contexts/       # React contexts (LanguageContext for i18n)
│   ├── lib/
│   │   ├── api.ts     # Axios client with JWT interceptors
│   │   └── utils.ts   # Utilities (cn helper for Tailwind)
│   ├── hooks/          # Custom React hooks
│   ├── stores/         # Zustand stores
│   └── types/          # TypeScript type definitions
└── messages/           # Translation files (en.json, zh.json, ja.json)
```

**Key Technologies:**
- **Framework**: Next.js 16 with App Router, configured for static export (`output: 'export'`)
- **UI**: shadcn/ui + Radix UI components with Tailwind CSS v4
- **State**: Zustand for global state, TanStack Query for server state
- **Forms**: react-hook-form + zod validation
- **i18n**: Custom React Context (`LanguageContext`) with UI-based language switching (no URL locale prefix)

**API Client** (`ui/src/lib/api.ts`):
- Axios instance with base URL `/api`
- Request interceptor: adds JWT token from localStorage
- Response interceptor: handles 401 (redirects to login)
- Token storage: localStorage for persistence

### Multi-language System

**Frontend**: Custom React Context-based i18n with three supported locales (en, zh, ja)
- Implementation: `LanguageContext` in `ui/src/contexts/LanguageContext.tsx`
- Language switching: UI-based switcher component (no URL locale prefix)
- Messages: `ui/messages/{locale}.json` files loaded dynamically
- Storage: User's language preference saved in localStorage
- Default locale: Chinese (zh)

**Key Pattern**: Unlike typical next-intl implementations, this project uses in-app language switching to maintain clean URLs without locale prefixes (e.g., `/servers` instead of `/zh/servers`).

**Backend**: Planned i18n for API error messages based on Accept-Language header

## Database Schema

**Tables** (see `internal/database/database.go`):

1. **servers** - Palworld server instances
   - Tracks: name, install path, ports (game, query, RCON), status, PID
   
2. **mods** - Workshop mods per server
   - Links to server, stores Workshop ID, install path, enabled status
   
3. **users** - Admin accounts
   - Stores: username, bcrypt password hash

**Migrations**: Schema is applied via SQL strings in `database.go:migrate()`

## Development Notes

### Frontend-Backend Integration

The frontend is **statically built and embedded** into the Go binary. This means:

1. Frontend changes require rebuilding: `cd ui && bun run build`
2. The Go binary reads from `embed.FS`, not disk (after build)
3. API calls use relative paths (`/api/*`) - no CORS needed
4. Frontend routing handled by Next.js static export; API routing by Gin

### JWT Authentication Flow

1. User logs in via `/api/auth/login`
2. Backend returns JWT token
3. Frontend stores token in localStorage
4. All subsequent requests include `Authorization: Bearer <token>` header
5. On 401, frontend clears token and redirects to login

### Adding New API Endpoints

1. Define handler in `internal/api/handlers.go`
2. Register route in `internal/api/router.go:RegisterRoutes()`
3. Use `protected` group for authenticated endpoints
4. Return JSON via `c.JSON(status, data)`

### Adding New Frontend Pages

1. Create page in `ui/src/app/your-page/page.tsx` (no `[locale]` directory needed)
2. Add `'use client'` directive if using client-side features
3. Use `useTranslations()` hook from `@/contexts/LanguageContext` for i18n
4. Add translations to `ui/messages/{en,zh,ja}.json`
5. API calls via `apiClient` from `ui/src/lib/api.ts`
6. Rebuild frontend: `cd ui && bun run build`

**Example page structure:**
```typescript
'use client'

import { useTranslations } from '@/contexts/LanguageContext';

export default function YourPage() {
  const t = useTranslations('yourSection');
  
  return <div>{t('key')}</div>;
}
```

### Configuration Changes

When adding new config fields:
1. Update struct in `internal/config/config.go` with yaml/env tags
2. Update `config.example.yaml` with example
3. Document in README files if user-facing

## Important Patterns

### Single Binary Distribution

The build creates a completely self-contained executable with no external dependencies (except SteamCMD for game server management). This is achieved through:
- Pure Go SQLite driver (no CGO)
- Embedded static files (no separate web server)
- Config via file or environment (no mandatory external configs)

### Configuration Priority

Always remember: **config.yaml > environment variables > defaults**

The config loader in `config.Load()` returns immediately if `config.yaml` exists, bypassing all environment variable checks.

### Static Export Constraints

Because Next.js uses `output: 'export'`:
- No server-side rendering (SSR)
- No API routes in Next.js (use Go backend)
- No `getServerSideProps` or Server Components that fetch data
- All pages are pre-rendered at build time

## Project Status

This is an **early-stage project**. Core structure is in place, but many features are stubbed:

**Implemented:**
- Basic project structure
- Configuration system with YAML support
- Database schema and migrations
- API routing skeleton
- Frontend structure with i18n
- Build system for single-binary distribution

**To Be Implemented:**
- Server lifecycle management (start/stop/restart)
- SteamCMD integration for server/mod installation
- JWT authentication middleware
- Real-time log streaming (SSE)
- System monitoring (CPU/memory)
- UI components and pages
- RCON command interface (optional P2 feature)

When implementing new features, follow the established patterns and maintain the single-binary architecture principle.
