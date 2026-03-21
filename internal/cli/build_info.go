package cli

import "strings"

var (
	version   = "dev"
	commit    = ""
	buildDate = ""
)

func SetBuildInfo(v, c, d string) {
	if strings.TrimSpace(v) != "" {
		version = v
	}
	if strings.TrimSpace(c) != "" {
		commit = c
	}
	if strings.TrimSpace(d) != "" {
		buildDate = d
	}
}

func ResetBuildInfo() {
	version = "dev"
	commit = ""
	buildDate = ""
}

func buildVersion() string {
	parts := make([]string, 0, 3)
	parts = append(parts, version)
	if commit != "" {
		parts = append(parts, "commit="+commit)
	}
	if buildDate != "" {
		parts = append(parts, "date="+buildDate)
	}

	return strings.Join(parts, " ")
}
