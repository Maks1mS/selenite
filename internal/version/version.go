package version

import (
	"fmt"
	"runtime"
)

var (
	Version   = "dev"
	GitCommit = "unknown"
)

func FormatVersion() string {
	return fmt.Sprintf("Version: %s\nGitCommit: %s\nGo: %s", Version, GitCommit, runtime.Version())
}
