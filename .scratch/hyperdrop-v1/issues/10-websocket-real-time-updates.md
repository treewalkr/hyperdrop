Status: ready-for-agent

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Real-time file system event broadcasting via WebSocket so all connected browsers see uploads and deletions without refreshing.

Backend:
- `GET /api/ws` endpoint that upgrades to WebSocket (requires same auth as other API endpoints)
- A goroutine-safe subscriber list (hub) that tracks connected clients
- When a file is uploaded (`POST /api/upload`), broadcast a `file_uploaded` event: `{"type": "file_uploaded", "file": {"name": "photo.jpg", "size": 4200000, "category": "img"}}`
- When a file is deleted (`DELETE /api/files/{path}`), broadcast a `file_deleted` event: `{"type": "file_deleted", "name": "photo.jpg"}`
- Handle client disconnect gracefully (remove from subscriber list)
- Hub runs as a goroutine started at server bootstrap

Frontend:
- Alpine.js connects to `/api/ws` on page load
- On `file_uploaded` event, add the file to the reactive file list (Files page)
- On `file_deleted` event, remove the file from the reactive list
- Auto-reconnect with exponential backoff if the WebSocket connection drops
- Show a subtle toast when a file is added by another user ("photo.jpg received from another device")

## Acceptance criteria

- [ ] `GET /api/ws` upgrades to WebSocket when authenticated
- [ ] Unauthenticated WebSocket connection is rejected
- [ ] File upload broadcasts `file_uploaded` to all connected WS clients
- [ ] File deletion broadcasts `file_deleted` to all connected WS clients
- [ ] Multiple connected browsers all receive events simultaneously
- [ ] Client disconnect is handled gracefully (no goroutine leak)
- [ ] Client-side auto-reconnects on connection drop
- [ ] Tests connect a WS client, trigger operations, verify broadcast JSON messages

## Blocked by

- Issue 04 (file upload streaming multipart)
- Issue 06 (file deletion)
