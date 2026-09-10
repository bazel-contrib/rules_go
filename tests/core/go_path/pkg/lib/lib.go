package lib

import (
	"C"
	"embed"
)

//go:embed embedded_src.txt
var embeddedSource string

//go:embed renamed_embedded_src.txt
var renamedEmbeddedSource string

//go:embed template/index.html.tmpl
var indexTmpl string

//go:embed directory
var directorySource embed.FS
