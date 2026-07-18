package main

// Build-time variables injected via -ldflags.
// Example:
//
//	go build -ldflags "-X main.Version=v1.0.0 -X main.BuildTime=2026-01-01T00:00:00Z -X main.GitCommit=abc1234"
var (
	// Version is the application version tag (e.g. v1.2.3).
	Version = "dev"

	// BuildTime is the UTC timestamp when the binary was compiled.
	BuildTime = "unknown"

	// GitCommit is the short git commit hash used for the build.
	GitCommit = "unknown"
)
