package client

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/derkajecht/Beatrice/src/client/tui"
	"gopkg.in/yaml.v3"
)

// EnvConfigVar is the environment variable consulted when no explicit config
// path is provided.
const EnvConfigVar = "BEATRICE_CONFIG"

// Config holds the client's runtime configuration: the visual theme and
// inactivity timeout.
type Config struct {
	Theme             tui.ThemeConfig
	InactivityTimeout time.Duration
}

// DefaultConfig returns the client's built-in defaults, matching the values
// that were hard-coded before configuration files were supported.
func DefaultConfig() Config {
	return Config{
		Theme:             tui.DefaultTheme(),
		InactivityTimeout: 120 * time.Second,
	}
}

// yamlTheme mirrors the YAML shape of the theme section. Fields are pointers
// so unset keys can be distinguished from empty values and merged over the
// defaults.
type yamlTheme struct {
	Border      *string `yaml:"border"`
	BorderFocus *string `yaml:"border_focus"`
	Accent      *string `yaml:"accent"`
	Text        *string `yaml:"text"`
	Muted       *string `yaml:"muted"`
	Faint       *string `yaml:"faint"`
	Active      *string `yaml:"active"`
	Away        *string `yaml:"away"`
	Error       *string `yaml:"error"`
}

type yamlBehavior struct {
	InactivityTimeout *durationValue `yaml:"inactivity_timeout"`
}

type defaultBehavior struct {
	InactivityTimeout string `yaml:"inactivity_timeout"`
}

type defaultConfig struct {
	Theme    tui.ThemeConfig `yaml:"theme"`
	Behavior defaultBehavior `yaml:"behavior"`
}

// yamlConfig mirrors the YAML shape of the whole config file.
type yamlConfig struct {
	Theme    yamlTheme    `yaml:"theme"`
	Behavior yamlBehavior `yaml:"behavior"`
}

// durationValue accepts either a plain integer (interpreted as seconds) or a
// string parseable by time.ParseDuration (e.g. "90s", "2m").
type durationValue time.Duration

func (d *durationValue) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return fmt.Errorf("inactivity_timeout must be a scalar, got %v", value.Kind)
	}
	raw := strings.TrimSpace(value.Value)
	if seconds, err := strconv.Atoi(raw); err == nil {
		*d = durationValue(time.Duration(seconds) * time.Second)
		return nil
	}
	dur, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("invalid inactivity_timeout %q: must be seconds or a duration like \"90s\"", raw)
	}
	*d = durationValue(dur)
	return nil
}

// resolveConfigPath determines which file to load. Precedence: explicit path,
// then $BEATRICE_CONFIG, then the default location under os.UserConfigDir().
// An empty result means "no config file to load".
func resolveConfigPath(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if env := os.Getenv(EnvConfigVar); env != "" {
		return env, nil
	}
	baseDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config directory: %w", err)
	}
	return filepath.Join(baseDir, "beatrice", "config.yml"), nil
}

// LoadConfig loads the client configuration. Behavior:
//
//   - Path resolution: explicit path, then $BEATRICE_CONFIG, then
//     os.UserConfigDir()/beatrice/config.yml.
//   - A missing file is not an error; the defaults are returned.
//   - Malformed YAML, unknown fields, invalid colors, and non-positive
//     behavior values are errors.
//   - Keys present in the file override their defaults; absent keys keep the
//     default values.
func LoadConfig(path string) (Config, error) {
	cfg := DefaultConfig()

	resolved, err := resolveConfigPath(path)
	if err != nil {
		return cfg, err
	}
	if resolved == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			if err := writeDefaultConfig(resolved, cfg); err != nil {
				return cfg, err
			}
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read config file %s: %w", resolved, err)
	}

	// An empty (or comment-only) file is a valid way to say "use defaults".
	if len(bytes.TrimSpace(data)) == 0 {
		return cfg, nil
	}

	var raw yamlConfig
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&raw); err != nil {
		return cfg, fmt.Errorf("failed to parse config file %s: %w", resolved, err)
	}

	if raw.Theme.Border != nil {
		cfg.Theme.Border = *raw.Theme.Border
	}
	if raw.Theme.BorderFocus != nil {
		cfg.Theme.BorderFocus = *raw.Theme.BorderFocus
	}
	if raw.Theme.Accent != nil {
		cfg.Theme.Accent = *raw.Theme.Accent
	}
	if raw.Theme.Text != nil {
		cfg.Theme.Text = *raw.Theme.Text
	}
	if raw.Theme.Muted != nil {
		cfg.Theme.Muted = *raw.Theme.Muted
	}
	if raw.Theme.Faint != nil {
		cfg.Theme.Faint = *raw.Theme.Faint
	}
	if raw.Theme.Active != nil {
		cfg.Theme.Active = *raw.Theme.Active
	}
	if raw.Theme.Away != nil {
		cfg.Theme.Away = *raw.Theme.Away
	}
	if raw.Theme.Error != nil {
		cfg.Theme.Error = *raw.Theme.Error
	}
	if raw.Behavior.InactivityTimeout != nil {
		cfg.InactivityTimeout = time.Duration(*raw.Behavior.InactivityTimeout)
	}
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func writeDefaultConfig(path string, cfg Config) error {
	data, err := yaml.Marshal(defaultConfig{
		Theme: cfg.Theme,
		Behavior: defaultBehavior{
			InactivityTimeout: cfg.InactivityTimeout.String(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write default config %s: %w", path, err)
	}
	return nil
}

// Validate checks that all theme colors are valid and all behavior values are
// positive. Empty color strings are allowed and mean "keep the default".
func (c Config) Validate() error {
	colors := map[string]string{
		"border":       c.Theme.Border,
		"border_focus": c.Theme.BorderFocus,
		"accent":       c.Theme.Accent,
		"text":         c.Theme.Text,
		"muted":        c.Theme.Muted,
		"faint":        c.Theme.Faint,
		"active":       c.Theme.Active,
		"away":         c.Theme.Away,
		"error":        c.Theme.Error,
	}
	for name, value := range colors {
		if err := validateColor(value); err != nil {
			return fmt.Errorf("invalid theme color %s: %w", name, err)
		}
	}
	if c.InactivityTimeout <= 0 {
		return fmt.Errorf("inactivity_timeout must be positive, got %s", c.InactivityTimeout)
	}
	return nil
}

// validateColor accepts an empty string (default), an ANSI palette index
// 0-255, or a #rrggbb hex color.
func validateColor(value string) error {
	if value == "" {
		return nil
	}
	if strings.HasPrefix(value, "#") {
		if len(value) != 7 {
			return fmt.Errorf("hex colors must be #rrggbb, got %q", value)
		}
		if _, err := strconv.ParseUint(value[1:], 16, 32); err != nil {
			return fmt.Errorf("invalid hex color %q", value)
		}
		return nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 255 {
		return fmt.Errorf("color must be an ANSI index 0-255 or #rrggbb, got %q", value)
	}
	return nil
}
