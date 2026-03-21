package cli

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

var (
	colorRed    = color.New(color.FgRed)
	colorGreen  = color.New(color.FgGreen)
	colorYellow = color.New(color.FgYellow)
)

func colorDiffPrefix(changeType string) string {
	switch changeType {
	case "add":
		return colorGreen.Sprint("+")
	case "remove":
		return colorRed.Sprint("-")
	default:
		return colorYellow.Sprint("~")
	}
}

func colorSeverity(severity string) string {
	switch severity {
	case "error":
		return colorRed.Sprint("error")
	case "warning":
		return colorYellow.Sprint("warning")
	default:
		return severity
	}
}

func colorSuccess(msg string) string {
	return colorGreen.Sprint(msg)
}

func fprintWarning(w io.Writer, msg string) {
	_, _ = fmt.Fprintf(w, "%s %s\n", colorYellow.Sprint("warning:"), msg)
}
