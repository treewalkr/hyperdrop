package static

import "embed"

//go:embed index.html files.html
var Assets embed.FS
