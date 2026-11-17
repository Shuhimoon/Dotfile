package main

import "fmt"

const modulePath = "github.com/shuhi/dotfiles"

// Set by GoReleaser: -X main.version=... -X main.commit=... -X main.date=...
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func versionString() string {
	return fmt.Sprintf("tools %s (commit %s, built %s)", version, commit, date)
}
