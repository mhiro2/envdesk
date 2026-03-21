package cli

import (
	"fmt"
	"io"
	"slices"

	"github.com/mhiro2/envdesk/internal/app"
)

func writeRekeyErrorSummary(w io.Writer, baseDir string, errs []app.RekeyError) error {
	if len(errs) == 0 {
		return nil
	}

	for _, rekeyErr := range errs {
		if _, err := fmt.Fprintf(w, "error: rekey %s: %s\n", formatProjectPath(baseDir, rekeyErr.Path), rekeyErr.Message); err != nil {
			return fmt.Errorf("write rekey output: %w", err)
		}
	}

	byMessage := make(map[string]int)
	for _, rekeyErr := range errs {
		byMessage[rekeyErr.Message]++
	}

	messages := make([]string, 0, len(byMessage))
	for message := range byMessage {
		messages = append(messages, message)
	}
	slices.Sort(messages)

	if _, err := fmt.Fprintf(w, "error: %d files failed during rekey\n", len(errs)); err != nil {
		return fmt.Errorf("write rekey output: %w", err)
	}
	for _, message := range messages {
		if _, err := fmt.Fprintf(w, "error: %d files: %s\n", byMessage[message], message); err != nil {
			return fmt.Errorf("write rekey output: %w", err)
		}
	}

	return nil
}
