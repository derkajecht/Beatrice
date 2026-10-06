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
	if cfg.InactivityTimeout != 120*time.Second {
		t.Errorf("InactivityTimeout = %s, want %s", cfg.InactivityTimeout, 120*time.Second)
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
}

func TestLoadConfigValid(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(t *testing.T, cfg Config)
	}{
		{
			name: "full behavior config",
			content: `
behavior:
  inactivity_timeout: 30
`,
			check: func(t *testing.T, cfg Config) {
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
behavior:
  inactivity_timeout: 45s
`,
			check: func(t *testing.T, cfg Config) {
				if cfg.InactivityTimeout != 45*time.Second {
					t.Errorf("InactivityTimeout = %s, want 45s", cfg.InactivityTimeout)
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
			content: "behavior:\n  inactivity_timeout: [unclosed\n",
			wantErr: "failed to parse config file",
		},
		{
			name:    "unknown field",
			content: "not_a_real_key: 1\n",
			wantErr: "failed to parse config file",
		},
		{
			name:    "legacy theme section rejected",
			content: "theme:\n  border: \"4\"\n",
			wantErr: "failed to parse config file",
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
