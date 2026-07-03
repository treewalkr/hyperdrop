package static

import "embed"

//go:embed index.html files.html player.html share.html shares.html clipboard.js
var Assets embed.FS
