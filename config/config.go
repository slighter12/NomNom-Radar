package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-viper/mapstructure/v2"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/slighter12/go-lib/database/postgres"
)

const (
	defaultPath                 = "."
	defaultMaxRequestBodySize   = "100KB"
	postgresMasterDSNEnvKey     = "POSTGRES_MASTER_DSN"
	defaultAccessTokenTTL       = 15 * time.Minute
	defaultRefreshTokenTTL      = 7 * 24 * time.Hour
	defaultOnboardingTokenTTL   = 10 * time.Minute
	defaultLinkingTokenTTL      = 10 * time.Minute
	defaultNotificationTimeout  = 10 * time.Second
	defaultDeviceCleanupTimeout = 5 * time.Minute
)

type Config struct {
	Env struct {
		Env         string `json:"env" yaml:"env"`
		ServiceName string `json:"serviceName" yaml:"serviceName"`
		Debug       bool   `json:"debug" yaml:"debug"`
		Log         Log    `json:"log" yaml:"log"`
	} `json:"env" yaml:"env"`

	HTTP struct {
		Port               int    `json:"port" yaml:"port"`
		MaxRequestBodySize string `json:"maxRequestBodySize" yaml:"maxRequestBodySize"`
		AllowedHost        string `json:"allowedHost" yaml:"allowedHost"`
		CloudflareSecret   string `json:"cloudflareSecret" yaml:"cloudflareSecret"`
		Timeouts           struct {
			ReadTimeout       time.Duration `json:"readTimeout" yaml:"readTimeout"`
			ReadHeaderTimeout time.Duration `json:"readHeaderTimeout" yaml:"readHeaderTimeout"`
			WriteTimeout      time.Duration `json:"writeTimeout" yaml:"writeTimeout"`
			IdleTimeout       time.Duration `json:"idleTimeout" yaml:"idleTimeout"`
		} `json:"timeouts" yaml:"timeouts"`
	} `json:"http" yaml:"http"`

	Postgres *postgres.DBConn `json:"postgres" yaml:"postgres" mapstructure:"postgres"`

	SecretKey struct {
		Access     string `json:"access" yaml:"access"`
		Refresh    string `json:"refresh" yaml:"refresh"`
		Onboarding string `json:"onboarding" yaml:"onboarding"`
		Linking    string `json:"linking" yaml:"linking"`
	} `json:"secretKey" yaml:"secretKey"`

	GoogleOAuth *GoogleOAuthConfig `json:"googleOAuth" yaml:"googleOAuth"`

	Auth *AuthConfig `json:"auth" yaml:"auth"`

	LoginThrottle *LoginThrottleConfig `json:"loginThrottle" yaml:"loginThrottle"`

	PasswordStrength *PasswordStrengthConfig `json:"passwordStrength" yaml:"passwordStrength"`

	// TestRoutes configuration for testing endpoints
	TestRoutes *TestRoutesConfig `json:"testRoutes" yaml:"testRoutes"`

	// LocationNotification configuration for location notification system
	LocationNotification *LocationNotificationConfig `json:"locationNotification" yaml:"locationNotification"`

	// Notification configuration for notification delivery behavior
	Notification *NotificationConfig `json:"notification" yaml:"notification"`

	// Firebase configuration for push notifications
	Firebase *FirebaseConfig `json:"firebase" yaml:"firebase"`

	// QRCode configuration for subscription QR codes
	QRCode *QRCodeConfig `json:"qrcode" yaml:"qrcode"`

	// PubSub configuration for event publishing
	PubSub *PubSubConfig `json:"pubsub" yaml:"pubsub"`

	// PMTiles configuration for serverless routing
	PMTiles *PMTilesConfig `json:"pmtiles" yaml:"pmtiles"`

	// DeviceCleanup configuration for stale device cleanup job
	DeviceCleanup *DeviceCleanupConfig `json:"deviceCleanup" yaml:"deviceCleanup"`
}

type GoogleOAuthConfig struct {
	ClientID string `json:"clientId" yaml:"clientId"`
	// Note: ClientSecret and RedirectURI are not needed for ID token verification
	// These are only needed for server-side OAuth flows, which we don't use
}

// AuthConfig defines authentication-related configuration
type AuthConfig struct {
	Argon2Memory        uint32        `json:"argon2Memory" yaml:"argon2Memory"`
	Argon2Iterations    uint32        `json:"argon2Iterations" yaml:"argon2Iterations"`
	Argon2Parallelism   uint8         `json:"argon2Parallelism" yaml:"argon2Parallelism"`
	Argon2MaxConcurrent int           `json:"argon2MaxConcurrent" yaml:"argon2MaxConcurrent"`
	MaxActiveSessions   int           `json:"maxActiveSessions" yaml:"maxActiveSessions"`
	AccessTokenTTL      time.Duration `json:"accessTokenTTL" yaml:"accessTokenTTL"`
	RefreshTokenTTL     time.Duration `json:"refreshTokenTTL" yaml:"refreshTokenTTL"`
	OnboardingTokenTTL  time.Duration `json:"onboardingTokenTTL" yaml:"onboardingTokenTTL"`
	LinkingTokenTTL     time.Duration `json:"linkingTokenTTL" yaml:"linkingTokenTTL"`
}

// LoginThrottleConfig defines progressive login throttling configuration.
type LoginThrottleConfig struct {
	MaxAttempts      int `json:"maxAttempts" yaml:"maxAttempts"`
	LockoutDecayDays int `json:"lockoutDecayDays" yaml:"lockoutDecayDays"`
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
	Pretty bool   `json:"pretty" yaml:"pretty"`
	Level  string `json:"level" yaml:"level"`
}

// TestRoutesConfig defines configuration for testing endpoints
type TestRoutesConfig struct {
	Enabled bool `json:"enabled" yaml:"enabled"`
}

// LocationNotificationConfig defines configuration for location notification system
type LocationNotificationConfig struct {
	UserMaxLocations     int     `json:"userMaxLocations" yaml:"userMaxLocations"`
	MerchantMaxLocations int     `json:"merchantMaxLocations" yaml:"merchantMaxLocations"`
	DefaultRadius        float64 `json:"defaultRadius" yaml:"defaultRadius"`
	MaxRadius            float64 `json:"maxRadius" yaml:"maxRadius"`
}

// NotificationConfig defines notification behavior configuration.
type NotificationConfig struct {
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// FirebaseConfig defines Firebase configuration for push notifications
type FirebaseConfig struct {
	ProjectID       string `json:"projectId" yaml:"projectId"`
	CredentialsPath string `json:"credentialsPath" yaml:"credentialsPath"`
}

// QRCodeConfig defines QR code generation configuration
type QRCodeConfig struct {
	ErrorCorrectionLevel string `json:"errorCorrectionLevel" yaml:"errorCorrectionLevel"`
}

// PubSubConfig defines Pub/Sub configuration for event publishing
type PubSubConfig struct {
	// Provider type: "local" for local HTTP or "google" for Google Pub/Sub
	Provider string `json:"provider" yaml:"provider"`

	// Google Cloud project ID (for google provider)
	ProjectID string `json:"projectId" yaml:"projectId"`

	// Pub/Sub topic ID (for google provider)
	TopicID string `json:"topicId" yaml:"topicId"`

	// Local HTTP endpoint for development (for local provider)
	LocalEndpoint string `json:"localEndpoint" yaml:"localEndpoint"`
}

// PMTilesConfig defines PMTiles routing configuration for notification runtime routing.
type PMTilesConfig struct {
	// Enable PMTiles-based routing
	Enabled bool `json:"enabled" yaml:"enabled"`

	// PMTiles source URL (local file path, HTTP URL, or GCS URL)
	Source string `json:"source" yaml:"source"`

	// Road layer name in the MVT tiles
	RoadLayer string `json:"roadLayer" yaml:"roadLayer"`

	// Zoom level for tile queries
	ZoomLevel int `json:"zoomLevel" yaml:"zoomLevel"`
}

// DeviceCleanupConfig defines cleanup-job runtime configuration.
type DeviceCleanupConfig struct {
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
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
			return nil, fmt.Errorf("os.Getwd: %w", err)
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

	existingConfigMap := koanfInstance.Raw()

	// Load environment variables
	if err := koanfInstance.Load(env.Provider(".", env.Opt{
		TransformFunc: func(k, v string) (string, any) {
			// Convert ENV_VAR_NAME to path and align each segment with existing YAML keys.
			// Example: POSTGRES_SSLMODE -> postgres.sslMode (not postgres.sslmode)
			key := canonicalizeEnvKey(k, existingConfigMap)

			return key, v
		},
	}), nil); err != nil {
		return nil, fmt.Errorf("load env variables failed: %w", err)
	}

	// Unmarshal into the config struct (case-insensitive to match env vars)
	if err := koanfInstance.UnmarshalWithConf("", cfg, koanf.UnmarshalConf{
		DecoderConfig: &mapstructure.DecoderConfig{
			Result:           cfg,
			WeaklyTypedInput: true,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				mapstructure.StringToTimeDurationHookFunc(),
			),
			MatchName: func(mapKey, fieldName string) bool {
				// Case-insensitive matching for env var overrides
				return strings.EqualFold(mapKey, fieldName)
			},
		},
	}); err != nil {
		return nil, fmt.Errorf("unmarshal %s config failed: %w", currEnv, err)
	}

	return cfg, nil
}

func New() (*Config, error) {
	cfg, err := LoadWithEnv[Config]("config", "config", "../config", "../../config")
	if err != nil {
		return nil, err
	}

	ApplyDefaults(cfg)

	// Build replicas from environment variables (POSTGRES_REPLICAS_0_HOST, POSTGRES_REPLICAS_0_PORT, etc.)
	cfg.Postgres.Replicas = buildReplicasFromEnv()
	if err := applyPostgresMasterDSNFromEnv(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// ApplyDefaults mutates cfg in-place to ensure all supported default values are present.
func ApplyDefaults(cfg *Config) {
	if cfg == nil {
		return
	}

	applyHTTPDefaults(cfg)
	applyAuthDefaults(cfg)
	applyLoginThrottleDefaults(cfg)
	applyLocationNotificationDefaults(cfg)
	applyNotificationDefaults(cfg)
	applyDeviceCleanupDefaults(cfg)
}

func applyHTTPDefaults(cfg *Config) {
	if strings.TrimSpace(cfg.HTTP.MaxRequestBodySize) != "" {
		return
	}

	cfg.HTTP.MaxRequestBodySize = defaultMaxRequestBodySize
}

func applyAuthDefaults(cfg *Config) {
	if cfg.Auth == nil {
		cfg.Auth = &AuthConfig{}
	}
	if cfg.Auth.Argon2Memory == 0 {
		cfg.Auth.Argon2Memory = 65536
	}
	if cfg.Auth.Argon2Iterations == 0 {
		cfg.Auth.Argon2Iterations = 3
	}
	if cfg.Auth.Argon2Parallelism == 0 {
		cfg.Auth.Argon2Parallelism = 1
	}
	if cfg.Auth.Argon2MaxConcurrent <= 0 {
		cfg.Auth.Argon2MaxConcurrent = 4
	}
	if cfg.Auth.AccessTokenTTL <= 0 {
		cfg.Auth.AccessTokenTTL = defaultAccessTokenTTL
	}
	if cfg.Auth.RefreshTokenTTL <= 0 {
		cfg.Auth.RefreshTokenTTL = defaultRefreshTokenTTL
	}
	if cfg.Auth.OnboardingTokenTTL <= 0 {
		cfg.Auth.OnboardingTokenTTL = defaultOnboardingTokenTTL
	}
	if cfg.Auth.LinkingTokenTTL <= 0 {
		cfg.Auth.LinkingTokenTTL = defaultLinkingTokenTTL
	}
}

func applyLoginThrottleDefaults(cfg *Config) {
	if cfg.LoginThrottle == nil {
		cfg.LoginThrottle = &LoginThrottleConfig{}
	}
	if cfg.LoginThrottle.MaxAttempts <= 0 {
		cfg.LoginThrottle.MaxAttempts = 5
	}
	if cfg.LoginThrottle.LockoutDecayDays <= 0 {
		cfg.LoginThrottle.LockoutDecayDays = 7
	}
}

func applyLocationNotificationDefaults(cfg *Config) {
	if cfg.LocationNotification == nil {
		cfg.LocationNotification = &LocationNotificationConfig{}
	}
	if cfg.LocationNotification.UserMaxLocations <= 0 {
		cfg.LocationNotification.UserMaxLocations = 5
	}
	if cfg.LocationNotification.MerchantMaxLocations <= 0 {
		cfg.LocationNotification.MerchantMaxLocations = 10
	}
	if cfg.LocationNotification.DefaultRadius <= 0 {
		cfg.LocationNotification.DefaultRadius = 1000
	}
	if cfg.LocationNotification.MaxRadius <= 0 {
		cfg.LocationNotification.MaxRadius = 5000
	}
}

func applyNotificationDefaults(cfg *Config) {
	if cfg.Notification == nil {
		cfg.Notification = &NotificationConfig{}
	}
	if cfg.Notification.Timeout <= 0 {
		cfg.Notification.Timeout = defaultNotificationTimeout
	}
}

func applyDeviceCleanupDefaults(cfg *Config) {
	if cfg.DeviceCleanup == nil {
		cfg.DeviceCleanup = &DeviceCleanupConfig{}
	}
	if cfg.DeviceCleanup.Timeout <= 0 {
		cfg.DeviceCleanup.Timeout = defaultDeviceCleanupTimeout
	}
}

func canonicalizeEnvKey(rawKey string, existing map[string]any) string {
	segments := strings.Split(strings.ToLower(rawKey), "_")
	canonical := make([]string, 0, len(segments))
	current := existing

	for _, segment := range segments {
		if segment == "" {
			continue
		}

		if matched, next, ok := findExistingSegment(current, segment); ok {
			canonical = append(canonical, matched)
			current = next
		} else {
			canonical = append(canonical, segment)
			current = nil
		}
	}

	return strings.Join(canonical, ".")
}

func findExistingSegment(current map[string]any, segment string) (matched string, next map[string]any, ok bool) {
	if len(current) == 0 {
		return "", nil, false
	}

	needle := normalizeToken(segment)
	for key, value := range current {
		if normalizeToken(key) != needle {
			continue
		}

		child, _ := value.(map[string]any)

		return key, child, true
	}

	return "", nil, false
}

func normalizeToken(s string) string {
	var normalized strings.Builder
	normalized.Grow(len(s))

	for _, r := range s {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		normalized.WriteRune(unicode.ToLower(r))
	}

	return normalized.String()
}

// buildReplicasFromEnv builds the replicas slice from environment variables.
// Environment variable format: POSTGRES_REPLICAS_{index}_{field}
// Example: POSTGRES_REPLICAS_0_HOST, POSTGRES_REPLICAS_0_PORT, POSTGRES_REPLICAS_0_USERNAME, POSTGRES_REPLICAS_0_PASSWORD
func buildReplicasFromEnv() []postgres.ConnectionConfig {
	var replicas []postgres.ConnectionConfig

	for i := 0; ; i++ {
		prefix := "POSTGRES_REPLICAS_" + strconv.Itoa(i) + "_"

		host := os.Getenv(prefix + "HOST")
		port := os.Getenv(prefix + "PORT")
		if host == "" || port == "" {
			// No more replicas or incomplete configuration.
			break
		}

		replica := postgres.ConnectionConfig{
			Host:     host,
			Port:     port,
			UserName: os.Getenv(prefix + "USERNAME"),
			Password: os.Getenv(prefix + "PASSWORD"),
		}

		replicas = append(replicas, replica)
	}

	return replicas
}

func applyPostgresMasterDSNFromEnv(cfg *Config) error {
	dsn := strings.TrimSpace(os.Getenv(postgresMasterDSNEnvKey))
	if dsn == "" {
		return nil
	}

	if cfg.Postgres == nil {
		return fmt.Errorf("%s is set but postgres config is nil", postgresMasterDSNEnvKey)
	}

	parsed, err := pgconn.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse %s: %w", postgresMasterDSNEnvKey, err)
	}

	cfg.Postgres.Master.Host = parsed.Host
	cfg.Postgres.Master.Port = strconv.FormatUint(uint64(parsed.Port), 10)
	cfg.Postgres.Master.UserName = parsed.User
	cfg.Postgres.Master.Password = parsed.Password

	if strings.TrimSpace(parsed.Database) != "" {
		cfg.Postgres.Database = parsed.Database
	}

	return nil
}
