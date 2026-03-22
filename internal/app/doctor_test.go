package app_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/mhiro2/envdesk/internal/app"
	"github.com/mhiro2/envdesk/internal/testutil/platformtest"
	"github.com/mhiro2/envdesk/internal/testutil/projecttest"
)

func TestDoctor_Healthy(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/.*\.env$
    age: []
`,
		".gitignore": "*.env.local\n*.local.env\n",
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=ENC[AES256_GCM,data:dev]\n",
		"env/api/stg.env": "APP_ENV=ENC[AES256_GCM,data:stg]\n",
	})

	prepareDoctorTools(t, true, true)
	gitInitRepo(t, root)
	gitAddAll(t, root)

	// Act
	result, err := app.Doctor(t.Context(), app.DoctorOptions{
		ConfigPath: filepath.Join(root, "envdesk.yaml"),
	})
	// Assert
	if err != nil {
		t.Fatalf("Doctor() error = %v, want nil", err)
	}
	if !result.Healthy {
		t.Fatalf("result.Healthy = %v, want true", result.Healthy)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("len(result.Findings) = %d, want 0", len(result.Findings))
	}
}

func TestDoctor_ValidationFailures(t *testing.T) {
	tests := []struct {
		name      string
		files     map[string]string
		withSOPS  bool
		withAge   bool
		wantCheck string
	}{
		{
			name: "invalid config",
			files: map[string]string{
				"envdesk.yaml": "version: [\n",
				".sops.yaml": `creation_rules:
  - path_regex: ^env/.*\.env$
    age: []
`,
				".gitignore": "*.env.local\n*.local.env\n",
			},
			withSOPS:  true,
			withAge:   true,
			wantCheck: "config",
		},
		{
			name: "invalid sops config",
			files: map[string]string{
				"envdesk.yaml": `version: 1
services:
  - name: api
    files:
      dev: env/api/dev.env
`,
				".sops.yaml":      "creation_rules: [\n",
				".gitignore":      "*.env.local\n*.local.env\n",
				"env/api/dev.env": "APP_ENV=dev\n",
			},
			withSOPS:  true,
			withAge:   true,
			wantCheck: "sops_config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := projecttest.WriteProject(t, tt.files)
			prepareDoctorTools(t, tt.withSOPS, tt.withAge)
			gitInitRepo(t, root)
			gitAddAll(t, root)

			// Act
			result, err := app.Doctor(t.Context(), app.DoctorOptions{
				ConfigPath: filepath.Join(root, "envdesk.yaml"),
			})
			// Assert
			if err != nil {
				t.Fatalf("Doctor() error = %v, want nil", err)
			}
			if result.Healthy {
				t.Fatal("result.Healthy = true, want false")
			}
			if !hasFinding(result.Findings, tt.wantCheck, app.SeverityError) {
				t.Fatalf("findings = %#v, want %s error", result.Findings, tt.wantCheck)
			}
		})
	}
}

func TestDoctor_UnhealthyState(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/.*\.env$
    age: []
`,
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=ENC[AES256_GCM,data:dev]\n",
		".env.local":      "APP_ENV=dev\nSECRET_KEY=plaintext\n",
	})

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("# intentionally incomplete\n"), 0o644); err != nil {
		t.Fatalf("write gitignore: %v", err)
	}
	if err := os.Chmod(filepath.Join(root, ".env.local"), 0o644); err != nil {
		t.Fatalf("chmod export file: %v", err)
	}

	prepareDoctorTools(t, false, false)
	gitInitRepo(t, root)
	gitAddAll(t, root)

	// Act
	result, err := app.Doctor(t.Context(), app.DoctorOptions{
		ConfigPath: filepath.Join(root, "envdesk.yaml"),
	})
	// Assert
	if err != nil {
		t.Fatalf("Doctor() error = %v, want nil", err)
	}
	if result.Healthy {
		t.Fatal("result.Healthy = true, want false")
	}
	if !hasFinding(result.Findings, "sops", app.SeverityError) {
		t.Fatalf("findings = %#v, want sops error", result.Findings)
	}
	if !hasFinding(result.Findings, "age", app.SeverityError) {
		t.Fatalf("findings = %#v, want age error", result.Findings)
	}
	if !hasFinding(result.Findings, "tracked_plaintext", app.SeverityError) {
		t.Fatalf("findings = %#v, want tracked plaintext error", result.Findings)
	}
	if !hasFinding(result.Findings, "gitignore", app.SeverityWarning) {
		t.Fatalf("findings = %#v, want gitignore warning", result.Findings)
	}
	if !platformtest.SupportsPermissionChecks() {
		return
	}
	if !hasFindingTarget(result.Findings, "permissions", ".env.local", app.SeverityWarning) {
		t.Fatalf("findings = %#v, want permission warning for .env.local", result.Findings)
	}
}

func TestDoctor_WarnsWhenOutsideGitRepository(t *testing.T) {
	// Arrange
	root := projecttest.WriteProject(t, doctorHealthyFiles())
	prepareDoctorTools(t, true, true)

	// Act
	result, err := app.Doctor(t.Context(), app.DoctorOptions{
		ConfigPath: filepath.Join(root, "envdesk.yaml"),
	})
	// Assert
	if err != nil {
		t.Fatalf("Doctor() error = %v, want nil", err)
	}
	if !result.Healthy {
		t.Fatalf("result.Healthy = %v, want true", result.Healthy)
	}
	if !hasFinding(result.Findings, "git", app.SeverityWarning) {
		t.Fatalf("findings = %#v, want git warning", result.Findings)
	}
	if hasFinding(result.Findings, "git", app.SeverityError) {
		t.Fatalf("findings = %#v, want no git error", result.Findings)
	}
}

func TestDoctor_AgeKeyFileValidation(t *testing.T) {
	tests := []struct {
		name         string
		prepareKey   func(t *testing.T)
		wantHealthy  bool
		wantAgeError bool
	}{
		{
			name: "usable key file",
			prepareKey: func(t *testing.T) {
				t.Helper()

				keyPath := filepath.Join(t.TempDir(), "keys.txt")
				if err := os.WriteFile(keyPath, []byte("AGE-SECRET-KEY-1EXAMPLE\n"), 0o600); err != nil {
					t.Fatalf("write age key file: %v", err)
				}
				t.Setenv("SOPS_AGE_KEY_FILE", keyPath)
			},
			wantHealthy: true,
		},
		{
			name: "empty key file",
			prepareKey: func(t *testing.T) {
				t.Helper()

				keyPath := filepath.Join(t.TempDir(), "keys.txt")
				if err := os.WriteFile(keyPath, nil, 0o600); err != nil {
					t.Fatalf("write age key file: %v", err)
				}
				t.Setenv("SOPS_AGE_KEY_FILE", keyPath)
			},
			wantAgeError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			root := projecttest.WriteProject(t, doctorHealthyFiles())
			prepareDoctorTools(t, true, false)
			tt.prepareKey(t)
			gitInitRepo(t, root)
			gitAddAll(t, root)

			// Act
			result, err := app.Doctor(t.Context(), app.DoctorOptions{
				ConfigPath: filepath.Join(root, "envdesk.yaml"),
			})
			// Assert
			if err != nil {
				t.Fatalf("Doctor() error = %v, want nil", err)
			}
			if result.Healthy != tt.wantHealthy {
				t.Fatalf("result.Healthy = %v, want %v", result.Healthy, tt.wantHealthy)
			}
			if hasFinding(result.Findings, "age", app.SeverityError) != tt.wantAgeError {
				t.Fatalf("findings = %#v, age error = %v, want %v", result.Findings, hasFinding(result.Findings, "age", app.SeverityError), tt.wantAgeError)
			}
		})
	}
}

func TestDoctor_SOPSConfigValidationDetails(t *testing.T) {
	tests := []struct {
		name         string
		sopsConfig   string
		wantSeverity app.Severity
	}{
		{
			name: "missing path regex",
			sopsConfig: `creation_rules:
  - age: []
`,
			wantSeverity: app.SeverityError,
		},
		{
			name: "age must be a sequence",
			sopsConfig: `creation_rules:
  - path_regex: ^env/.*\.env$
    age: age1recipient
`,
			wantSeverity: app.SeverityError,
		},
		{
			name: "unmatched creation rules",
			sopsConfig: `creation_rules:
  - path_regex: ^secrets/.*\.env$
    age: []
`,
			wantSeverity: app.SeverityWarning,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			files := doctorHealthyFiles()
			files[".sops.yaml"] = tt.sopsConfig

			root := projecttest.WriteProject(t, files)
			prepareDoctorTools(t, true, true)
			gitInitRepo(t, root)
			gitAddAll(t, root)

			// Act
			result, err := app.Doctor(t.Context(), app.DoctorOptions{
				ConfigPath: filepath.Join(root, "envdesk.yaml"),
			})
			// Assert
			if err != nil {
				t.Fatalf("Doctor() error = %v, want nil", err)
			}
			if !hasFinding(result.Findings, "sops_config", tt.wantSeverity) {
				t.Fatalf("findings = %#v, want sops_config %s", result.Findings, tt.wantSeverity)
			}
		})
	}
}

func TestDoctor_GitignoreCoveragePatterns(t *testing.T) {
	tests := []struct {
		name        string
		gitignore   string
		wantWarning bool
	}{
		{
			name:      "double star env local",
			gitignore: "**/.env.local\n",
		},
		{
			name:      "root anchored local exports",
			gitignore: "/env/**/*.local\n",
		},
		{
			name:      "envrc local",
			gitignore: ".envrc.local\n",
		},
		{
			name:      "service specific ignore",
			gitignore: "env/api/.env.local\n",
		},
		{
			name:        "unrelated rule",
			gitignore:   "node_modules/\n",
			wantWarning: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			files := doctorHealthyFiles()
			files[".gitignore"] = tt.gitignore

			root := projecttest.WriteProject(t, files)
			prepareDoctorTools(t, true, true)
			gitInitRepo(t, root)
			gitAddAll(t, root)

			// Act
			result, err := app.Doctor(t.Context(), app.DoctorOptions{
				ConfigPath: filepath.Join(root, "envdesk.yaml"),
			})
			// Assert
			if err != nil {
				t.Fatalf("Doctor() error = %v, want nil", err)
			}
			if hasFinding(result.Findings, "gitignore", app.SeverityWarning) != tt.wantWarning {
				t.Fatalf("findings = %#v, gitignore warning = %v, want %v", result.Findings, hasFinding(result.Findings, "gitignore", app.SeverityWarning), tt.wantWarning)
			}
		})
	}
}

func doctorHealthyFiles() map[string]string {
	return map[string]string{
		"envdesk.yaml": `version: 1
services:
  - name: api
    schema: env.schema/api.yaml
    files:
      dev: env/api/dev.env
      stg: env/api/stg.env
`,
		".sops.yaml": `creation_rules:
  - path_regex: ^env/.*\.env$
    age: []
`,
		".gitignore": "*.env.local\n*.local.env\n",
		"env.schema/api.yaml": `keys:
  APP_ENV:
    required: true
    type: enum
    values: [dev, stg]
    secret: false
`,
		"env/api/dev.env": "APP_ENV=ENC[AES256_GCM,data:dev]\n",
		"env/api/stg.env": "APP_ENV=ENC[AES256_GCM,data:stg]\n",
	}
}

func prepareDoctorTools(t *testing.T, withSOPS, withAge bool) {
	t.Helper()

	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("look up git: %v", err)
	}

	binDir := t.TempDir()
	writeDoctorExecutable(t, binDir, "git", doctorProxyScript(realGit))
	if withSOPS {
		writeDoctorExecutable(t, binDir, "sops", doctorNoopScript())
	}
	if withAge {
		writeDoctorExecutable(t, binDir, "age", doctorNoopScript())
	}

	homeDir := t.TempDir()
	t.Setenv("PATH", binDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(homeDir, ".config"))
	t.Setenv("APPDATA", filepath.Join(homeDir, "AppData", "Roaming"))
	t.Setenv("SOPS_AGE_KEY_FILE", "")
}

func writeDoctorExecutable(t *testing.T, dir, name, content string) {
	t.Helper()

	path := filepath.Join(dir, doctorExecutableName(name))
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write executable %s: %v", path, err)
	}
}

func doctorExecutableName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".cmd"
	}

	return name
}

func doctorProxyScript(realPath string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("@echo off\r\n\"%s\" %%*\r\n", realPath)
	}

	return fmt.Sprintf("#!/bin/sh\nexec %s \"$@\"\n", shellQuote(realPath))
}

func doctorNoopScript() string {
	if runtime.GOOS == "windows" {
		return "@echo off\r\nexit /b 0\r\n"
	}

	return "#!/bin/sh\nexit 0\n"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func gitInitRepo(t *testing.T, root string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "-C", root, "init")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
}

func gitAddAll(t *testing.T, root string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", "-C", root, "add", ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
}

func hasFinding(findings []app.DoctorFinding, check string, severity app.Severity) bool {
	for _, finding := range findings {
		if finding.Check == check && finding.Severity == severity {
			return true
		}
	}

	return false
}

func hasFindingTarget(findings []app.DoctorFinding, check, target string, severity app.Severity) bool {
	for _, finding := range findings {
		if finding.Check == check && finding.Target == target && finding.Severity == severity {
			return true
		}
	}

	return false
}
