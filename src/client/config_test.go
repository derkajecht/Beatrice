package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeTempConfig writes the given content to a temp file and returns its path.
func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}
	return path
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("defaults must be valid, got error: %v", err)
	}
	want := Config{
		Theme:             cfg.Theme, // color strings checked below via YAML round-trip
		InactivityTimeout: 120 * time.Second,
	}
	if cfg.InactivityTimeout != want.InactivityTimeout {
		t.Errorf("InactivityTimeout = %s, want %s", cfg.InactivityTimeout, want.InactivityTimeout)
	}
	colors := map[string]string{
		"border": cfg.Theme.Border, "border_focus": cfg.Theme.BorderFocus,
		"accent": cfg.Theme.Accent, "text": cfg.Theme.Text,
		"muted": cfg.Theme.Muted, "faint": cfg.Theme.Faint,
		"active": cfg.Theme.Active, "away": cfg.Theme.Away,
		"error": cfg.Theme.Error,
	}
	for name, value := range colors {
		if value == "" {
			t.Errorf("default theme color %s is empty", name)
		}
		if err := validateColor(value); err != nil {
			t.Errorf("default theme color %s = %q is invalid: %v", name, value, err)
		}
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	tests := []struct {
		name string
		path string
	}{
		{name: "explicit path does not exist", path: filepath.Join(t.TempDir(), "nope", "config.yml")},
		{name: "empty path", path: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure env var and default location do not interfere.
			t.Setenv(EnvConfigVar, filepath.Join(t.TempDir(), "env-config.yml"))
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("HOME", t.TempDir())

			cfg, err := LoadConfig(tt.path)
			if err != nil {
				t.Fatalf("LoadConfig(%q) error = %v, want nil", tt.path, err)
			}
			want := DefaultConfig()
			if cfg.InactivityTimeout != want.InactivityTimeout {
				t.Errorf("missing file must yield defaults: got %+v, want %+v", cfg, want)
			}
			if cfg.Theme != want.Theme {
				t.Errorf("missing file must yield default theme: got %+v, want %+v", cfg.Theme, want.Theme)
			}
		})
	}
}

func TestLoadConfigCreatesMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yml")
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v, want nil", err)
	}
	if cfg != DefaultConfig() {
		t.Fatalf("generated config returned %+v, want defaults %+v", cfg, DefaultConfig())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("generated config was not readable: %v", err)
	}
	if !strings.Contains(string(data), "inactivity_timeout: 2m0s") {
		t.Errorf("generated config missing inactivity timeout: %s", data)
	}
	if !strings.Contains(string(data), "border_focus: \"13\"") {
		t.Errorf("generated config missing theme: %s", data)
	}
}

func TestLoadConfigValid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "full config with ANSI and hex colors",
			content: `
theme:
  border: "4"
  border_focus: "#ff00aa"
  accent: "13"
  text: "#ffffff"
  muted: "7"
  faint: "0"
  active: "10"
  away: "11"
  error: "#ff0000"
behavior:
  inactivity_timeout: 30
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.Theme.Border != "4" {
					t.Errorf("Border = %q, want \"4\"", cfg.Theme.Border)
				}
				if cfg.Theme.BorderFocus != "#ff00aa" {
					t.Errorf("BorderFocus = %q, want \"#ff00aa\"", cfg.Theme.BorderFocus)
				}
				if cfg.Theme.Text != "#ffffff" {
					t.Errorf("Text = %q, want \"#ffffff\"", cfg.Theme.Text)
				}
				if cfg.Theme.Error != "#ff0000" {
					t.Errorf("Error = %q, want \"#ff0000\"", cfg.Theme.Error)
				}
				if cfg.InactivityTimeout != 30*time.Second {
					t.Errorf("InactivityTimeout = %s, want 30s", cfg.InactivityTimeout)
				}
			},
		},
		{
			name: "duration string form",
			content: `
behavior:
  inactivity_timeout: 2m30s
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.InactivityTimeout != 150*time.Second {
					t.Errorf("InactivityTimeout = %s, want 2m30s", cfg.InactivityTimeout)
				}
			},
		},
		{
			name: "partial config merges over defaults",
			content: `
theme:
  accent: "12"
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.Theme.Accent != "12" {
					t.Errorf("Accent = %q, want \"12\"", cfg.Theme.Accent)
				}
				want := DefaultConfig()
				if cfg.Theme.Border != want.Theme.Border {
					t.Errorf("Border = %q, want default %q", cfg.Theme.Border, want.Theme.Border)
				}
				if cfg.Theme.Error != want.Theme.Error {
					t.Errorf("Error = %q, want default %q", cfg.Theme.Error, want.Theme.Error)
				}
				if cfg.InactivityTimeout != want.InactivityTimeout {
					t.Errorf("InactivityTimeout = %s, want default %s", cfg.InactivityTimeout, want.InactivityTimeout)
				}
			},
		},
		{
			name:    "empty file uses defaults",
			content: "",
			check: func(t *testing.T, cfg Config) {
				want := DefaultConfig()
				if cfg != want {
					t.Errorf("empty file: got %+v, want defaults %+v", cfg, want)
				}
			},
		},
		{
			name:    "ANSI color boundary 0 and 255",
			content: "theme:\n  faint: \"0\"\n  active: \"255\"\n",
			check: func(t *testing.T, cfg Config) {
				if cfg.Theme.Faint != "0" || cfg.Theme.Active != "255" {
					t.Errorf("boundary colors not preserved: faint=%q active=%q", cfg.Theme.Faint, cfg.Theme.Active)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := LoadConfig(writeTempConfig(t, tt.content))
			if err != nil {
				t.Fatalf("LoadConfig() error = %v, want nil", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name:    "malformed yaml",
			content: "theme:\n  border: [unclosed\n",
			wantErr: "failed to parse config file",
		},
		{
			name:    "unknown field",
			content: "not_a_real_key: 1\n",
			wantErr: "failed to parse config file",
		},
		{
			name:    "unknown theme field",
			content: "theme:\n  not_a_color: \"3\"\n",
			wantErr: "failed to parse config file",
		},
		{
			name:    "color out of ansi range",
			content: "theme:\n  border: \"256\"\n",
			wantErr: "invalid theme color border",
		},
		{
			name:    "negative ansi color",
			content: "theme:\n  border: \"-1\"\n",
			wantErr: "invalid theme color border",
		},
		{
			name:    "non-numeric color",
			content: "theme:\n  border: \"red\"\n",
			wantErr: "invalid theme color border",
		},
		{
			name:    "hex color too short",
			content: "theme:\n  border: \"#12345\"\n",
			wantErr: "invalid theme color border",
		},
		{
			name:    "hex color non-hex digits",
			content: "theme:\n  border: \"#zzzzzz\"\n",
			wantErr: "invalid theme color border",
		},
		{
			name:    "zero inactivity timeout",
			content: "behavior:\n  inactivity_timeout: 0\n",
			wantErr: "inactivity_timeout must be positive",
		},
		{
			name:    "negative inactivity timeout",
			content: "behavior:\n  inactivity_timeout: -1s\n",
			wantErr: "inactivity_timeout must be positive",
		},
		{
			name:    "garbage duration",
			content: "behavior:\n  inactivity_timeout: banana\n",
			wantErr: "invalid inactivity_timeout",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadConfig(writeTempConfig(t, tt.content))
			if err == nil {
				t.Fatalf("LoadConfig() error = nil, want error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("LoadConfig() error = %q, want containing %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestLoadConfigPathResolution(t *testing.T) {
	envPath := writeTempConfig(t, "behavior:\n  inactivity_timeout: 111\n")
	t.Run("env var used when no explicit path", func(t *testing.T) {
		t.Setenv(EnvConfigVar, envPath)
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(\"\") error = %v, want nil", err)
		}
		if cfg.InactivityTimeout != 111*time.Second {
			t.Errorf("InactivityTimeout = %s, want 111s (from $BEATRICE_CONFIG)", cfg.InactivityTimeout)
		}
	})

	explicitPath := writeTempConfig(t, "behavior:\n  inactivity_timeout: 222\n")
	t.Run("explicit path wins over env var", func(t *testing.T) {
		t.Setenv(EnvConfigVar, envPath)
		cfg, err := LoadConfig(explicitPath)
		if err != nil {
			t.Fatalf("LoadConfig(explicit) error = %v, want nil", err)
		}
		if cfg.InactivityTimeout != 222*time.Second {
			t.Errorf("InactivityTimeout = %s, want 222s (explicit path should win)", cfg.InactivityTimeout)
		}
	})

	t.Run("default user config dir location", func(t *testing.T) {
		t.Setenv(EnvConfigVar, "")
		home := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", home)
		t.Setenv("HOME", home)
		defDir := filepath.Join(home, "beatrice")
		if err := os.MkdirAll(defDir, 0o755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}
		defPath := filepath.Join(defDir, "config.yml")
		if err := os.WriteFile(defPath, []byte("behavior:\n  inactivity_timeout: 333\n"), 0o600); err != nil {
			t.Fatalf("failed to write default config: %v", err)
		}
		cfg, err := LoadConfig("")
		if err != nil {
			t.Fatalf("LoadConfig(\"\") error = %v, want nil", err)
		}
		if cfg.InactivityTimeout != 333*time.Second {
			t.Errorf("InactivityTimeout = %s, want 333s (from default location)", cfg.InactivityTimeout)
		}
	})
}

func TestValidateColor(t *testing.T) {
	tests := []struct {
		value   string
		wantErr bool
	}{
		{value: "", wantErr: false},
		{value: "0", wantErr: false},
		{value: "255", wantErr: false},
		{value: "13", wantErr: false},
		{value: "256", wantErr: true},
		{value: "-1", wantErr: true},
		{value: "abc", wantErr: true},
		{value: "12x", wantErr: true},
		{value: "#000000", wantErr: false},
		{value: "#FF00aa", wantErr: false},
		{value: "#12345", wantErr: true},
		{value: "#1234567", wantErr: true},
		{value: "#12345g", wantErr: true},
		{value: "123456", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			err := validateColor(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateColor(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
			}
		})
	}
}
