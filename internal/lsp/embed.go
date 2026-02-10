package lsp

import "embed"

//go:embed all:bundle
var bundle embed.FS

// Compile-time check: fail the build if the LSP bundle has not been generated.
// Run `make build` to clone and compile the language-server into internal/lsp/bundle/.
//
//go:embed bundle/out/server.js
var _ string
