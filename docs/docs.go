package docs

import "embed"

// Docs embed docs static files
//
//go:embed static
var Docs embed.FS
