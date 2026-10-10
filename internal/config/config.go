package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the complete DataMask configuration.
type Config struct {
	Version  int                    `yaml:"version"`
	Source   DatabaseConfig         `yaml:"source"`
	Target   DatabaseConfig         `yaml:"target"`
	Settings Settings               `yaml:"settings"`
	Tables   map[string]TableConfig `yaml:"tables"`
	Presets  map[string]PresetConfig `yaml:"presets,omitempty"`
}

// DatabaseConfig holds database connection details.
type DatabaseConfig struct {
	Driver   string `yaml:"driver"`   // postgres, mysql, mariadb
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	SSLMode  string `yaml:"sslmode,omitempty"`
	Schema   string `yaml:"schema,omitempty"`
}

// Settings defines global processing flags and behaviour.
type Settings struct {
	Deterministic        bool     `yaml:"deterministic"`
	DeterministicKeyEnv  string   `yaml:"deterministic_key_env,omitempty"`
	PreservePrimaryKeys  bool     `yaml:"preserve_primary_keys"`
	StopOnError          bool     `yaml:"stop_on_error"`
	BatchSize            int      `yaml:"batch_size"`
	Workers              int      `yaml:"workers"`
	SubsetLimit          *int64   `yaml:"subset_limit,omitempty"`
	SubsetPercent        *float64 `yaml:"subset_percent,omitempty"`
	Preset               string   `yaml:"preset,omitempty"`
	ProtectedEnvironments []string `yaml:"protected_environments"`
	ExcludeTables        []string `yaml:"exclude_tables,omitempty"`
	IncludeTables        []string `yaml:"include_tables,omitempty"`
	DevelopmentAuth      DevAuthConfig `yaml:"development_auth,omitempty"`
}

// DevAuthConfig configures global development credentials injection.
type DevAuthConfig struct {
	Enabled     bool   `yaml:"enabled"`
	PasswordEnv string `yaml:"password_env,omitempty"`
	FixedHash   string `yaml:"fixed_hash,omitempty"`
}

// TableConfig contains per-table masking strategy directives.
type TableConfig struct {
	Strategy string                  `yaml:"strategy,omitempty"` // e.g. "truncate", "preserve"
	Columns  map[string]ColumnConfig `yaml:"columns,omitempty"`
	Where    string                  `yaml:"where,omitempty"`
	Limit    *int64                  `yaml:"limit,omitempty"`
}

// ColumnConfig specifies strategy and options for a particular column.
type ColumnConfig struct {
	Strategy      string            `yaml:"strategy"`
	Deterministic *bool             `yaml:"deterministic,omitempty"`
	Params        map[string]any    `yaml:"params,omitempty"`
	Format        string            `yaml:"format,omitempty"`
}

// PresetConfig allows defining reusable column transformation bundles.
type PresetConfig struct {
	Strategy string         `yaml:"strategy"`
	Params   map[string]any `yaml:"params,omitempty"`
}

var envVarPattern = regexp.MustCompile(`\$\{([A-Za-z0-9_]+)(?::-([^}]*))?\}`)

// ExpandEnv substitutes ${VAR} or ${VAR:-default} patterns in text.
func ExpandEnv(input string) string {
	return envVarPattern.ReplaceAllStringFunc(input, func(m string) string {
		parts := envVarPattern.FindStringSubmatch(m)
		if len(parts) < 2 {
			return m
		}
		varName := parts[1]
		val, exists := os.LookupEnv(varName)
		if exists && val != "" {
			return val
		}
		if len(parts) >= 3 && parts[2] != "" {
			return parts[2] // default value
		}
		return val
	})
}

// Load loads, expands environment variables, and parses a config file.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file %s: %w", path, err)
	}

	expanded := ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing yaml configuration: %w", err)
	}

	cfg.SetDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// SetDefaults assigns sensible safe defaults.
func (c *Config) SetDefaults() {
	if c.Version == 0 {
		c.Version = 1
	}
	if c.Settings.BatchSize <= 0 {
		c.Settings.BatchSize = 5000
	}
	if c.Settings.Workers <= 0 {
		c.Settings.Workers = 4
	}
	if c.Settings.DeterministicKeyEnv == "" {
		c.Settings.DeterministicKeyEnv = "DATAMASK_TRANSFORM_KEY"
	}
	if len(c.Settings.ProtectedEnvironments) == 0 {
		c.Settings.ProtectedEnvironments = []string{
			"prod", "production", "live", "main", "primary", "master",
		}
	}
	if c.Source.Schema == "" {
		if c.Source.Driver == "postgres" {
			c.Source.Schema = "public"
		}
	}
	if c.Target.Schema == "" {
		if c.Target.Driver == "postgres" {
			c.Target.Schema = "public"
		}
	}
	if c.Tables == nil {
		c.Tables = make(map[string]TableConfig)
	}
}

// Validate ensures structural correctness and security constraints.
func (c *Config) Validate() error {
	if c.Source.Driver == "" {
		return fmt.Errorf("source.driver is required")
	}
	validDrivers := map[string]bool{"postgres": true, "mysql": true, "mariadb": true, "sqlite": true}
	if !validDrivers[strings.ToLower(c.Source.Driver)] {
		return fmt.Errorf("unsupported source driver: %s", c.Source.Driver)
	}

	if c.Target.Driver != "" && !validDrivers[strings.ToLower(c.Target.Driver)] {
		return fmt.Errorf("unsupported target driver: %s", c.Target.Driver)
	}

	return nil
}

// IsProtectedTarget returns true if the target database name indicates a production system.
func (c *Config) IsProtectedTarget(targetName string) bool {
	low := strings.ToLower(targetName)
	for _, env := range c.Settings.ProtectedEnvironments {
		if strings.Contains(low, strings.ToLower(env)) {
			return true
		}
	}
	return false
}

// ParseDSN parses standard connection URL strings (postgres://, mysql://, sqlite://)
func ParseDSN(raw string) (DatabaseConfig, error) {
	var cfg DatabaseConfig
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "postgres://") || strings.HasPrefix(raw, "postgresql://") {
		cfg.Driver = "postgres"
		u, err := url.Parse(raw)
		if err != nil {
			return cfg, err
		}
		cfg.Host = u.Hostname()
		if u.Port() != "" {
			var p int
			fmt.Sscanf(u.Port(), "%d", &p)
			cfg.Port = p
		} else {
			cfg.Port = 5432
		}
		if u.User != nil {
			cfg.Username = u.User.Username()
			pass, _ := u.User.Password()
			cfg.Password = pass
		}
		cfg.Database = strings.TrimPrefix(u.Path, "/")
		cfg.SSLMode = u.Query().Get("sslmode")
		if cfg.SSLMode == "" {
			cfg.SSLMode = "disable"
		}
		cfg.Schema = u.Query().Get("schema")
		if cfg.Schema == "" {
			cfg.Schema = "public"
		}
		return cfg, nil
	}

	if strings.HasPrefix(raw, "mysql://") || strings.HasPrefix(raw, "mariadb://") {
		cfg.Driver = "mysql"
		u, err := url.Parse(raw)
		if err != nil {
			return cfg, err
		}
		cfg.Host = u.Hostname()
		if u.Port() != "" {
			var p int
			fmt.Sscanf(u.Port(), "%d", &p)
			cfg.Port = p
		} else {
			cfg.Port = 3306
		}
		if u.User != nil {
			cfg.Username = u.User.Username()
			pass, _ := u.User.Password()
			cfg.Password = pass
		}
		cfg.Database = strings.TrimPrefix(u.Path, "/")
		return cfg, nil
	}

	if strings.HasPrefix(raw, "sqlite://") || strings.HasPrefix(raw, "sqlite3://") {
		cfg.Driver = "sqlite"
		path := strings.TrimPrefix(raw, "sqlite://")
		path = strings.TrimPrefix(path, "sqlite3://")
		cfg.Database = path
		return cfg, nil
	}

	return cfg, fmt.Errorf("unrecognized database url scheme: %s (expected postgres://, mysql://, or sqlite://)", raw)
}
