package crypto

import (
	"fmt"
	"strings"
)

type CommandError struct {
	Tool     string
	Action   string
	Target   string
	ExitCode int
	Stderr   string
	Err      error
}

func (e *CommandError) Error() string {
	base := fmt.Sprintf("run %s %s %q", e.Tool, e.Action, e.Target)

	switch {
	case e.ExitCode != 0 && e.Stderr != "":
		return fmt.Sprintf("%s: exit code %d: %s", base, e.ExitCode, e.Stderr)
	case e.ExitCode != 0:
		return fmt.Sprintf("%s: exit code %d", base, e.ExitCode)
	case e.Stderr != "":
		return fmt.Sprintf("%s: %s", base, e.Stderr)
	default:
		return fmt.Sprintf("%s: %v", base, e.Err)
	}
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

func sanitizeStderr(stderr []byte) string {
	text := strings.TrimSpace(string(stderr))
	if text == "" {
		return ""
	}

	return strings.Join(strings.Fields(text), " ")
}
