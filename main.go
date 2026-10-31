// Command envy is an encrypted, multi-environment secrets manager for
// project .env files.
package main

import (
	"os"
	"runtime/debug"

	"github.com/anastanveer653/envy/cmd"
)

// Set by GoReleaser via -ldflags.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	if version == "dev" {
		// `go install ...@v1.2.3` records the module version.
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			version = bi.Main.Version
		}
	}
	os.Exit(cmd.Execute(cmd.BuildInfo{Version: version, Commit: commit, Date: date}))
}
