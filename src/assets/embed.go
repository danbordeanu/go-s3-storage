package assets

import "embed"

//go:embed css/*.css js/*.js
var StaticFS embed.FS
