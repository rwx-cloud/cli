package lsp

import "embed"

//go:embed bundle/server.js
var bundle embed.FS

// Compile-time check: fail the build if the LSP bundle has not been generated.
// Run `make build` to clone and compile the language-server into internal/lsp/bundle/.
//
//go:embed bundle/server.js
var _ string
