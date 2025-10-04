package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"github.com/slighter12/go-lib/database/postgres"
)

const defaultPath = "."

type Config struct {
	Env struct {
		Env         string `json:"env" yaml:"env"`
		ServiceName string `json:"serviceName" yaml:"serviceName"`
		Debug       bool   `json:"debug" yaml:"debug"`
		Log         Log    `json:"log" yaml:"log"`
	} `json:"env" yaml:"env"`

	HTTP struct {
		Port     int `json:"port" yaml:"port"`
		Timeouts struct {
			ReadTimeout       time.Duration `json:"readTimeout" yaml:"readTimeout"`
			ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`
			WriteTimeout      time.Duration `json:"writeTimeout" yaml:"writeTimeout"`
			IdleTimeout       time.Duration `json:"idleTimeout" yaml:"idleTimeout"`
		} `json:"timeouts" yaml:"timeouts"`
	} `json:"http" yaml:"http"`

	Postgres *postgres.DBConn `json:"postgres" yaml:"postgres" mapstructure:"postgres"`

	SecretKey struct {
		Access  string `json:"access" yaml:"access"`
		Refresh string `json:"refresh" yaml:"refresh"`
	} `json:"secretKey" yaml:"secretKey"`

	GoogleOAuth *GoogleOAuthConfig `json:"googleOAuth" yaml:"googleOAuth"`

	Auth *AuthConfig `json:"auth" yaml:"auth"`

	PasswordStrength *PasswordStrengthConfig `json:"passwordStrength" yaml:"passwordStrength"`

	// TestRoutes configuration for testing endpoints
	TestRoutes *TestRoutesConfig `json:"testRoutes" yaml:"testRoutes"`
}

type GoogleOAuthConfig struct {
	ClientID string `json:"clientId" yaml:"clientId"`
	// Note: ClientSecret and RedirectURI are not needed for ID token verification
	// These are only needed for server-side OAuth flows, which we don't use
}

// AuthConfig defines authentication-related configuration
type AuthConfig struct {
	BcryptCost int `json:"bcryptCost" yaml:"bcryptCost"`
}

// PasswordStrengthConfig defines password strength requirements
type PasswordStrengthConfig struct {
	MinLength        int  `json:"minLength" yaml:"minLength"`
	RequireUppercase bool `json:"requireUppercase" yaml:"requireUppercase"`
	RequireLowercase bool `json:"requireLowercase" yaml:"requireLowercase"`
	RequireNumbers   bool `json:"requireNumbers" yaml:"requireNumbers"`
	RequireSpecial   bool `json:"requireSpecial" yaml:"requireSpecial"`
	MaxLength        int  `json:"maxLength" yaml:"maxLength"`
}

type Log struct {
	Pretty       bool          `json:"pretty" yaml:"pretty"`
	Level        string        `json:"level" yaml:"level"`
	Path         string        `json:"path" yaml:"path"`
	MaxAge       time.Duration `json:"maxAge" yaml:"maxAge"`
	RotationTime time.Duration `json:"rotationTime" yaml:"rotationTime"`
}

// TestRoutesConfig defines configuration for testing endpoints
type TestRoutesConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// LoadWithEnv loads .yaml files through koanf.
func LoadWithEnv[T any](currEnv string, configPath ...string) (*T, error) {
	cfg := new(T)
	koanfInstance := koanf.New(".")

	// Build list of paths to search for config file
	searchPaths := []string{defaultPath}
	if len(configPath) != 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "os.Getwd")
		}
		for _, path := range configPath {
			abs := filepath.Join(pwd, path)
			searchPaths = append(searchPaths, abs)
		}
	}

	// Try to find and load the config file
	var configFile string
	var found bool
	for _, path := range searchPaths {
		candidate := filepath.Join(path, currEnv+".yaml")
		if _, err := os.Stat(candidate); err == nil {
			configFile = candidate
			found = true

			break
		}
	}

	if !found {
		return nil, fmt.Errorf("config file %s.yaml not found in any search path", currEnv)
	}

	// Load YAML config file
	if err := koanfInstance.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return nil, fmt.Errorf("read %s config failed: %w", currEnv, err)
	}

	// Load environment variables
	if err := koanfInstance.Load(env.Provider(".", env.Opt{
		TransformFunc: func(k, v string) (string, any) {
			// Convert ENV_VAR_NAME to env.var.name
			key := strings.ReplaceAll(strings.ToLower(k), "_", ".")

			return key, v
		},
	}), nil); err != nil {
		return nil, fmt.Errorf("load env variables failed: %w", err)
	}

	// Unmarshal into the config struct
	if err := koanfInstance.Unmarshal("", cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s config failed: %w", currEnv, err)
	}

	return cfg, nil
}

func New() (*Config, error) {
	return LoadWithEnv[Config]("config", "config", "../connfig", "../../config")
}
