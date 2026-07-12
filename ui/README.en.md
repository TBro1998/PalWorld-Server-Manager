# Palworld Server Manager - Frontend

> English | [中文](./README.md) | [日本語](./README.ja.md)

Next.js 16 frontend with App Router and static export for embedding in Go binary.

## Tech Stack

- **Framework**: Next.js 16 (App Router)
- **UI Components**: shadcn/ui + Radix UI
- **Styling**: Tailwind CSS v4
- **State Management**: Zustand
- **Data Fetching**: TanStack Query + axios
- **Forms**: react-hook-form + zod
- **i18n**: next-intl (supports en, zh, ja)

## Directory Structure

```
ui/
├── src/
│   ├── app/[locale]/  # App Router pages (internationalized)
│   ├── components/    # React components
│   ├── lib/          # Utilities and API client
│   ├── hooks/        # Custom React hooks
│   ├── stores/       # Zustand stores
│   ├── types/        # TypeScript types
│   └── i18n/         # i18n configuration
└── messages/         # Translation files (en.json, zh.json, ja.json)
```

## Development

```bash
pnpm install
pnpm run dev
```

Open [http://localhost:3000](http://localhost:3000)

## Building for Production

```bash
pnpm run build
```

This creates a static export in the `out/` directory, which is embedded into the Go binary.

## Internationalization

The app supports three languages:
- English (en)
- Chinese (zh)
- Japanese (ja)

Add translations in `messages/{locale}.json` and they'll be automatically picked up.

## API Integration

The frontend connects to the Go backend API at `/api`. The API client in `src/lib/api.ts` handles:
- JWT token management
- Request/response interceptors
- Automatic token refresh
- Error handling
