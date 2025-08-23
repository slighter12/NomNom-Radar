package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/slighter12/go-lib/database/postgres"
	"github.com/spf13/viper"
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

	GoogleOAuth struct {
		ClientID     string `json:"clientId" yaml:"clientId"`
		ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
		RedirectURI  string `json:"redirectUri" yaml:"redirectUri"`
		Scopes       string `json:"scopes" yaml:"scopes"`
	} `json:"googleOAuth" yaml:"googleOAuth"`

	Auth *AuthConfig `json:"auth" yaml:"auth"`

	PasswordStrength *PasswordStrengthConfig `json:"passwordStrength" yaml:"passwordStrength"`
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

// LoadWithEnv is a loads .yaml files through viper.
func LoadWithEnv[T any](currEnv string, configPath ...string) (*T, error) {
	cfg := new(T)
	configCtl := viper.New()
	configCtl.SetConfigName(currEnv)
	configCtl.SetConfigType("yaml")
	configCtl.AddConfigPath(defaultPath) // For Ops to deploy, but recommend consistent with the local environment later.
	if len(configPath) != 0 {
		pwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "os.Getwd")
		}
		for _, path := range configPath {
			abs := filepath.Join(pwd, path)
			configCtl.AddConfigPath(abs)
		}
	}
	configCtl.AutomaticEnv()
	configCtl.SetEnvKeyReplacer(strings.NewReplacer(",", "_"))

	if err := configCtl.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read %s config failed: %w", currEnv, err)
	}

	if err := configCtl.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal %s config failed: %w", currEnv, err)
	}

	return cfg, nil
}

func New() (*Config, error) {
	return LoadWithEnv[Config]("config", "config", "../connfig", "../../config")
}
