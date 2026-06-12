Status: ready-for-human

## Parent

`.scratch/hyperdrop-v1/PRD.md`

## What to build

Replace the placeholder Files page (`files.html`) with the full Aurora production UI for browsing, downloading, and deleting files.

The page includes:
- **Aurora animated background**: same four-blob canvas as the Send page
- **Glass header**: brand wordmark, Send/Files nav (active on Files), sort pills (Newest/Name/Size with glass pill + gradient active state), search box (glass pill with magnifying icon), file count stat in monospace
- **Table view** (default): sortable table with glass sticky header, checkbox column (select-all in header), file name with type badge, size column, date column (relative time), "from" column with green dot indicator, hover-revealed download/delete action buttons
- **Grid view**: auto-fill card grid, each card has a colored preview header, file type badge, "from" tag, checkbox overlay, hover-revealed action bar, card body with name + metadata
- **Feed view**: centered timeline with date dividers, avatar initials, sender name + timestamp, file chip with type icon + name + size, hover-revealed actions
- **Variant switcher**: floating glass pill at bottom center with arrow buttons to toggle Table/Grid/Feed
- **Confirmation modal**: glass backdrop + panel for delete confirmation
- **Toast notifications**: same system as Send page
- **Empty state**: centered muted text ("No files yet") when directory is empty
- **Multi-select**: checkboxes on each row/card, select-all in table header, selected state with violet background tint

Alpine.js fetches from `GET /api/files` on load, handles sort/search/filter client-side, wires download links to `GET /api/files/{path}`, wires delete to `DELETE /api/files/{path}` with confirmation modal.

All CSS tokens match DESIGN.md. Mobile-responsive: hide sort pills and stats, stack cards, full-width toasts.

This issue requires human visual review before merge.

## Acceptance criteria

- [ ] Table view renders files with all columns (checkbox, name+badge, size, date, from, actions)
- [ ] Grid view renders file cards with preview panels and hover action bars
- [ ] Feed view renders timeline with date dividers and file chips
- [ ] Variant switcher toggles between Table/Grid/Feed
- [ ] Sort pills sort by name, date, size (toggle ascending/descending on re-click)
- [ ] Search box filters files by name, extension, and source device
- [ ] File count and total size stat updates with search/filter
- [ ] Download button triggers file download
- [ ] Delete button opens confirmation modal, then calls DELETE endpoint
- [ ] Select-all checkbox selects/deselects all visible files
- [ ] Empty state shows when no files exist
- [ ] CSS tokens match DESIGN.md values
- [ ] Mobile layout is usable at 375px width

## Blocked by

- Issue 05 (file listing and download)
- Issue 06 (file deletion)
