package main

import "os"

// getEnvWithFallback checks the primary env var first, falling back to the
// legacy name for backwards compatibility during the CAPTAIN_ → RWX_TEST_ migration.
func getEnvWithFallback(primary, fallback string) string {
	if v := os.Getenv(primary); v != "" {
		return v
	}
	return os.Getenv(fallback)
}
