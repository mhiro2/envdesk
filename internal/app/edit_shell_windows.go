//go:build windows

package app

func currentEditorShell() editorShell {
	return editorShell{
		Command:      "cmd.exe",
		Args:         []string{"/d", "/s", "/c"},
		PathArgument: `"%` + editPathEnvVar + `%"`,
	}
}
