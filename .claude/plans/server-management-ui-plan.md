# Server Management UI Implementation Plan

## Current State Summary

### Backend Status
- ✅ Server model and database schema complete
- ✅ API endpoints registered (`/api/servers` with CRUD + lifecycle operations)
- ❌ All handlers return 501 Not Implemented - need actual implementation
- ❌ No repository or service layer exists
- ✅ SteamCMD auto-download/installation fully functional
- ❌ No code to use SteamCMD for Palworld server installation (app ID: 2394010)

### Frontend Status
- ✅ Basic layout with language switching (en/zh/ja)
- ✅ API client configured with JWT auth interceptors
- ✅ Translation files comprehensive with all needed keys
- ✅ Dependencies ready: react-hook-form, zod, TanStack Query, Zustand
- ❌ No UI component library installed (CLAUDE.md mentions shadcn/ui but not present)
- ❌ No pages, forms, dialogs, or feature components exist
- ❌ No routing between pages

### Key Requirement
Default server directory: `Server/{id}` under current working directory, with option to select custom directory.

## Implementation Strategy

This plan follows a phased approach, building from backend data layer up through API to frontend UI.

---

## Phase 1: Backend - Server CRUD Operations

### 1.1 Implement Database Operations in Handlers

**File**: `internal/api/handlers.go`

**ListServers** - Get all servers from database:
```go
- Query: SELECT * FROM servers ORDER BY created_at DESC
- Return: JSON array of server objects
- Status: 200 OK
```

**CreateServer** - Create new server instance:
```go
- Parse request body: {name, installPath}
- Validate: name required, installPath defaults to "Server/{id}"
- Insert into database with default values:
  - status: "stopped"
  - port: 8211 (default)
  - queryPort: 27015 (default)
  - rconPort: 25575 (default)
  - rconEnabled: false
- Return: Created server object with generated ID
- Status: 201 Created
```

**GetServer** - Get single server by ID:
```go
- Parse ID from URL params
- Query: SELECT * FROM servers WHERE id = ?
- Return: Server object or 404 if not found
```

**UpdateServer** - Update server configuration:
```go
- Parse ID and request body
- Update allowed fields: name, port, queryPort, rconPort, rconEnabled
- Return: Updated server object
```

**DeleteServer** - Delete server:
```go
- Parse ID from URL params
- Check if server is running (status != "stopped") - reject if running
- DELETE FROM servers WHERE id = ?
- Return: 204 No Content
```

### 1.2 Request Validation

Add request structs for validation:
```go
type CreateServerRequest struct {
    Name        string `json:"name" binding:"required"`
    InstallPath string `json:"installPath"`
}

type UpdateServerRequest struct {
    Name        string `json:"name"`
    Port        int    `json:"port"`
    QueryPort   int    `json:"queryPort"`
    RCONPort    int    `json:"rconPort"`
    RCONEnabled bool   `json:"rconEnabled"`
}
```

Use Gin's `ShouldBindJSON` for automatic validation.

---

## Phase 2: Backend - SteamCMD Integration for Palworld Server Installation

### 2.1 Create SteamCMD Execution Utility

**New File**: `internal/steamcmd/install.go`

Implement function to execute SteamCMD commands:
```go
func InstallPalworldServer(installPath string, steamCmdPath string) error
```

**Implementation details:**
- Use `os/exec` to run SteamCMD executable
- Command template: `steamcmd +login anonymous +app_update 2394010 validate +quit`
- Set working directory to installPath
- Capture stdout/stderr for progress tracking
- Return error if installation fails

Reference: https://docs.palworldgame.com/getting-started/deploy-dedicated-server

### 2.2 Add Install Endpoint

**Endpoint**: `POST /api/servers/:id/install`

**Handler logic:**
1. Get server by ID from database
2. Check server status - must be "stopped" (not already installing/running)
3. Create install directory if doesn't exist
4. Update server status to "installing"
5. Call `InstallPalworldServer()` in a goroutine
6. Update status to "stopped" when complete (or "error" if failed)
7. Return 202 Accepted immediately (installation continues in background)

**Response:**
```json
{
  "message": "Server installation started",
  "serverId": 1,
  "status": "installing"
}
```

### 2.3 Installation Status Tracking

**Option A (Simple):** Poll server status via `GET /api/servers/:id`
**Option B (Advanced):** Add SSE endpoint for real-time progress updates

Recommend Option A for initial implementation - simpler and sufficient.

---

## Phase 3: Frontend - UI Component Foundation

### 3.1 UI Component Library Decision

**Issue**: CLAUDE.md mentions shadcn/ui + Radix UI, but they are NOT installed.

**Options:**
- **A**: Install shadcn/ui (recommended in project docs)
- **B**: Build minimal custom components with existing Tailwind + lucide-react

**Recommendation**: Option B (custom components)
- Project already has minimal setup without shadcn/ui
- Only need 4-5 components for this feature
- Faster than setting up entire component library
- Can add shadcn/ui later if more complex UI needs arise

### 3.2 Core Components to Build

**Files to create in** `ui/src/components/ui/`:

1. **Button.tsx** - Basic button with variants (primary, secondary, destructive)
2. **Input.tsx** - Text input with label and error message support
3. **Dialog.tsx** - Modal dialog with backdrop and close handling
4. **Card.tsx** - Container for server cards
5. **Badge.tsx** - Status indicators (running, stopped, installing, error)

**Pattern**: Follow existing Tailwind patterns, use `cn()` utility from `lib/utils.ts`

---

## Phase 4: Frontend - Server Management UI

### 4.1 Server List Page

**File**: `ui/src/app/servers/page.tsx`

**Layout:**
- Page header with "Servers" title and "Add Server" button
- Grid of server cards (responsive: 1 col mobile, 2-3 cols desktop)
- Empty state when no servers exist
- Loading state during initial fetch

**Data Fetching:**
- Use TanStack Query: `useQuery({ queryKey: ['servers'], queryFn: () => apiClient.get('/servers') })`
- Auto-refetch every 5 seconds to update server statuses

### 4.2 Server Card Component

**File**: `ui/src/components/ServerCard.tsx`

**Display:**
- Server name
- Status badge (color-coded: green=running, gray=stopped, yellow=installing, red=error)
- Install path
- Action buttons (conditionally shown based on status)

**Actions:**
- Install button (shown when status="stopped" and server not yet installed)
- Start button (shown when status="stopped" and installed)
- Stop button (shown when status="running")
- Restart button (shown when status="running")
- Delete button (always shown, confirms before delete)

### 4.3 Add Server Dialog

**File**: `ui/src/components/AddServerDialog.tsx`

**Form Fields:**
1. **Server Name** (text input, required)
2. **Install Path** (text input with browse button)
   - Default value: `Server/{nextId}` (calculate from server count + 1)
   - Browse button: triggers directory picker

**Directory Picker:**
- Use `<input type="file" webkitdirectory />` for browser-based directory selection
- Note: This only works in modern browsers, provide manual text input as fallback
- Display selected path in the input field

**Form Handling:**
- Use react-hook-form with zod validation
- Submit: POST to `/api/servers` with {name, installPath}
- On success: close dialog, refetch server list, show success message
- On error: display error message in dialog

### 4.4 Install Server Flow

**User Journey:**
1. User clicks "Add Server" button
2. Fills in name and optionally selects custom directory
3. Clicks "Create" - server is created in database with status="stopped"
4. Dialog closes, new server card appears in list
5. User clicks "Install" button on the server card
6. Frontend calls `POST /api/servers/:id/install`
7. Server card shows "installing" status with spinner
8. Frontend polls `GET /api/servers/:id` every 3 seconds to check status
9. When status changes to "stopped", installation is complete
10. User can now click "Start" to run the server

---

## Phase 5: Integration & Polish

### 5.1 API Client Functions

**File**: `ui/src/lib/api.ts`

Add typed API functions:
```typescript
export const serversApi = {
  list: () => apiClient.get<Server[]>('/servers'),
  create: (data: CreateServerData) => apiClient.post<Server>('/servers', data),
  get: (id: number) => apiClient.get<Server>(`/servers/${id}`),
  delete: (id: number) => apiClient.delete(`/servers/${id}`),
  install: (id: number) => apiClient.post(`/servers/${id}/install`),
  start: (id: number) => apiClient.post(`/servers/${id}/start`),
  stop: (id: number) => apiClient.post(`/servers/${id}/stop`),
  restart: (id: number) => apiClient.post(`/servers/${id}/restart`),
}
```

### 5.2 TypeScript Types

**File**: `ui/src/types/server.ts`

```typescript
export interface Server {
  id: number
  name: string
  installPath: string
  port: number
  queryPort: number
  rconPort: number
  rconEnabled: boolean
  status: 'stopped' | 'running' | 'installing' | 'error'
  pid: number
  createdAt: string
  updatedAt: string
}

export interface CreateServerData {
  name: string
  installPath?: string
}
```

### 5.3 Error Handling

**Backend:**
- Return appropriate HTTP status codes (400 for validation, 404 for not found, 500 for server errors)
- Include error messages in JSON response: `{"error": "descriptive message"}`
- Validate server exists before operations
- Prevent operations on servers in wrong state (e.g., can't start while installing)

**Frontend:**
- Display error messages from API in toast notifications or inline in dialogs
- Handle network errors gracefully
- Show loading states during async operations
- Disable buttons during operations to prevent duplicate requests

### 5.4 Navigation and Routing

**Current Issue:** App has no routing between pages

**Solution:** Add navigation link in layout
- Update `ui/src/app/layout.tsx` to include navigation bar
- Add link to `/servers` page
- Use Next.js `<Link>` component for client-side navigation

---

## Implementation Order

### Recommended Sequence:

**Step 1:** Backend CRUD operations (Phase 1)
- Implement ListServers, CreateServer, GetServer handlers
- Test with curl/Postman before building UI

**Step 2:** SteamCMD integration (Phase 2)
- Implement InstallPalworldServer function
- Add /install endpoint
- Test SteamCMD download manually

**Step 3:** UI base components (Phase 3)
- Build Button, Input, Dialog, Card, Badge components
- Create simple test page to verify components work

**Step 4:** Server list and add functionality (Phase 4.1-4.3)
- Build server list page
- Build add server dialog
- Connect to backend APIs (list, create)

**Step 5:** Install functionality (Phase 4.4)
- Add install button and handling
- Implement status polling
- Test full flow: create → install → wait → complete

**Step 6:** Polish and error handling (Phase 5)
- Add proper error messages
- Improve loading states
- Add navigation
- Test edge cases

---

## Potential Issues and Mitigations

### Issue 1: Directory Picker Browser Compatibility

**Problem:** `webkitdirectory` attribute not supported in all browsers

**Solution:**
- Provide manual text input as primary method
- Add note that directory must exist and be writable
- Consider adding backend validation of path accessibility

### Issue 2: Long-Running SteamCMD Installation

**Problem:** Palworld server download is ~10-15GB, takes several minutes

**Solution:**
- Use goroutine for async installation (already planned)
- Poll status every 3-5 seconds (not too frequent)
- Show progress indicator on frontend
- Consider adding installation logs endpoint in future

### Issue 3: Path Handling Cross-Platform

**Problem:** Windows vs Linux path separators and formats

**Solution:**
- Use Go's `filepath.Join()` for path construction
- Store paths in database with forward slashes
- Convert to OS-specific format when executing commands

### Issue 4: Concurrent Installations

**Problem:** Multiple servers installing simultaneously might overload disk/network

**Solution:**
- Initial implementation: no restriction (simple)
- Future: Add queue system or limit concurrent installations

---

## Testing Considerations

### Backend Testing
- Test CRUD operations with various inputs
- Test validation (missing required fields, invalid IDs)
- Test SteamCMD integration with actual download
- Verify database constraints (e.g., can't delete running server)
- Test concurrent requests

### Frontend Testing
- Test form validation (empty name, invalid paths)
- Test API error handling (network errors, 404, 500)
- Test loading and success states
- Test responsive layout on different screen sizes
- Verify i18n translations display correctly

### Integration Testing
- Complete flow: create server → install → check status
- Test with multiple servers
- Test rapid create/delete operations
- Verify UI updates when backend status changes

---

## Summary

This plan implements a complete server management UI with the following features:

**Backend:**
- Server CRUD operations with database persistence
- SteamCMD integration for automatic Palworld server installation
- RESTful API endpoints with proper error handling

**Frontend:**
- Server list page with cards showing status and actions
- Add server dialog with name input and directory picker
- Install functionality with status polling
- Basic UI components (Button, Input, Dialog, Card, Badge)

**Default Behavior:**
- New servers default to `Server/{id}` directory under current working directory
- Users can select custom directory via directory picker or manual input
- Installation happens asynchronously in background via SteamCMD

**Implementation Priority:**
Backend first (enables testing via API tools), then UI components, then integration. Total estimate: 15-20 files to create/modify across backend and frontend.

