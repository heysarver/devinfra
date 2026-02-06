package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Set via ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Version string `json:"version" yaml:"version"`
	Commit  string `json:"commit" yaml:"commit"`
	Date    string `json:"date" yaml:"date"`
	Go      string `json:"go" yaml:"go"`
	OS      string `json:"os" yaml:"os"`
	Arch    string `json:"arch" yaml:"arch"`
}

func Get() Info {
	v := Version
	c := Commit

	// Fallback to debug.ReadBuildInfo for go install builds
	if v == "dev" {
		if bi, ok := debug.ReadBuildInfo(); ok {
			if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
				v = bi.Main.Version
			}
			for _, s := range bi.Settings {
				if s.Key == "vcs.revision" && len(s.Value) >= 7 {
					c = s.Value[:7]
				}
			}
		}
	}

	return Info{
		Version: v,
		Commit:  c,
		Date:    Date,
		Go:      runtime.Version(),
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
	}
}

func (i Info) String() string {
	return fmt.Sprintf("devinfra %s (%s) built %s with %s on %s/%s", i.Version, i.Commit, i.Date, i.Go, i.OS, i.Arch)
}
