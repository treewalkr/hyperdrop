package static

import "embed"

//go:embed index.html files.html player.html share.html shares.html
var Assets embed.FS
