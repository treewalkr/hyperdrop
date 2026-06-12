Status: ready-for-human

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Replace the placeholder Send page (`index.html`) with the full Aurora production UI. This is the visual showcase of the app — the glassmorphism design system from DESIGN.md applied to the upload experience.

The page includes:
- **Aurora animated background**: four CSS blobs (blob-1 through blob-4) drifting on 18–25s cycles at 120px blur over the `#080b1a` canvas
- **Glass header**: brand wordmark "HyperDrop" with gradient text, Send/Files nav links with glass pill styling, active state on Send
- **Nebula dropzone** (Variant A as default): 260px circular dropzone with rotating conic-gradient ring, pulsing glow halo, inner frosted-glass circle. Handles drag-and-drop and click-to-browse.
- **Body drag-over state**: when dragging files over the page, aurora blobs intensify (blur tightens, opacity increases) and a dashed violet border overlay appears
- **File cards**: glass panel cards showing file icon (color-coded by type), filename, size metadata, progress bar with gradient fill, speed readout. Actions: pause/cancel during upload, download/copy-link/delete after completion
- **Toast notifications**: slide-in glass toasts for success (teal left border), error (coral), info (violet) — auto-dismiss after 2.5s
- **Confirmation modal**: glass backdrop + glass panel for cancel/delete confirmations

Alpine.js manages all reactive state: file list, upload progress (polled or event-driven), drag state, toasts, modal visibility.

All CSS design tokens must match DESIGN.md exactly (colors, spacing, radii, typography, effects). The prototype variant switcher (floating pill) is removed — only the Nebula dropzone is rendered.

Mobile-responsive: dropzone scales down, card layout adapts, toasts go full-width.

This issue requires human visual review before merge.

## Acceptance criteria

- [ ] Aurora background renders with four animated blobs at correct colors, sizes, and timings
- [ ] Glass header matches DESIGN.md: gradient brand, glass nav pills, active state
- [ ] Nebula dropzone handles drag-and-drop and click-to-browse
- [ ] Body drag-over state intensifies blobs and shows dashed border overlay
- [ ] File cards show upload progress with gradient progress bar, speed, size metadata
- [ ] File type icons use the five-color language (doc purple, img teal, vid coral, zip amber, file muted)
- [ ] Toast notifications appear for upload success/error/info events
- [ ] Confirmation modal appears before cancel/delete actions
- [ ] Alpine.js wires all interactions to the backend API
- [ ] Mobile layout is usable at 375px width
- [ ] CSS tokens match DESIGN.md values

## Blocked by

- Issue 04 (file upload streaming multipart)
