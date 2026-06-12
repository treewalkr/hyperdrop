# HyperDrop — Domain Glossary

## Core Concepts

- **HyperDrop**: A CLI tool that starts an HTTP server for local file transfer and management over LAN.
- **root directory**: The directory specified by the user via CLI argument (or current directory by default). All file operations are sandboxed to this directory and its descendants.
- **token**: A string that grants access to the server. Auto-generated on startup (random 8 characters) unless user provides one via `--token`. Validated via query parameter or session cookie.
- **session**: After a browser presents a valid token, the server sets a cookie. Subsequent requests use the cookie instead of passing the token in every URL.

## Users & Access

- **LAN access**: The server binds to `0.0.0.0` by default, making it reachable from other devices on the same local network. Override with `--host 127.0.0.1` for local-only access.
- **network URL**: The address printed on startup that includes the server's LAN IP and token (e.g., `http://192.168.1.42:8080/?token=a7x9k2m4`).

## File Operations (v1)

- **upload**: Sending a file from a browser to the server. Uses multipart form upload with streaming write directly to disk. No temp file buffering.
- **download**: Sending a file from the server to a browser. Triggered by clicking a file in the Files page.
- **delete**: Removing a file or folder from the root directory via the web portal.
- **navigate**: Browsing into subdirectories within the root directory. The current path is shown as a breadcrumb.

## UI Concepts

- **Send page**: The upload interface. Contains a dropzone for drag-and-drop file uploads. Maps to `index.html`.
- **Files page**: The file browser interface. Shows files and folders in the current directory using table, grid, or feed views. Maps to `files.html`.
- **breadcrumb**: A navigation element showing the current directory path as clickable segments (e.g., `Home > Photos > Vacation`).
- **view variant**: The Files page supports three layout modes — Table (A), Grid (B), Feed (C) — toggled by the variant switcher.

## Security

- **path sandboxing**: All file operations are validated to ensure the resolved path stays within the root directory. Paths containing `..` traversal or absolute paths that escape the root are rejected.
- **max upload size**: An optional `--max-size` flag that rejects uploads exceeding the specified byte limit. Default is unlimited.

## Architecture

- **static assets**: HTML, CSS, and JavaScript files embedded into the Go binary at compile time via `embed.FS`. No external file serving.
- **WebSocket**: A persistent connection at `/api/ws` that pushes file system events (file added, file deleted) to all connected browsers for real-time updates.
