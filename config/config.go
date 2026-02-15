package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/pkg/errors"
	"github.com/slighter12/go-lib/database/postgres"
)

const (
	defaultPath               = "."
	defaultMaxRequestBodySize = "100KB"
)

type Config struct {
	Env struct {
		Env         string `json:"env" yaml:"env"`
		ServiceName string `json:"serviceName" yaml:"serviceName"`
		Debug       bool   `json:"debug" yaml:"debug"`
		Log         Log    `json:"log" yaml:"log"`
	} `json:"env" yaml:"env"`

	HTTP struct {
		Port int `json:"port" yaml:"port"`
		MaxRequestBodySize string `json:"maxRequestBodySize" yaml:"maxRequestBodySize"`
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

	// LocationNotification configuration for location notification system
	LocationNotification *LocationNotificationConfig `json:"locationNotification" yaml:"locationNotification"`

	// Firebase configuration for push notifications
	Firebase *FirebaseConfig `json:"firebase" yaml:"firebase"`

	// QRCode configuration for subscription QR codes
	QRCode *QRCodeConfig `json:"qrcode" yaml:"qrcode"`

	// Routing configuration for the routing engine
	Routing *RoutingConfig `json:"routing" yaml:"routing"`

	// PubSub configuration for event publishing
	PubSub *PubSubConfig `json:"pubsub" yaml:"pubsub"`

	// PMTiles configuration for serverless routing
	PMTiles *PMTilesConfig `json:"pmtiles" yaml:"pmtiles"`
}

type GoogleOAuthConfig struct {
	ClientID string `json:"clientId" yaml:"clientId"`
	// Note: ClientSecret and RedirectURI are not needed for ID token verification
	// These are only needed for server-side OAuth flows, which we don't use
}

// AuthConfig defines authentication-related configuration
type AuthConfig struct {
	BcryptCost        int `json:"bcryptCost" yaml:"bcryptCost"`
	MaxActiveSessions int `json:"maxActiveSessions" yaml:"maxActiveSessions"`
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

// FirebaseConfig defines Firebase configuration for push notifications
type FirebaseConfig struct {
	ProjectID       string `json:"projectId" yaml:"projectId"`
	CredentialsPath string `json:"credentialsPath" yaml:"credentialsPath"`
}

// QRCodeConfig defines QR code generation configuration
type QRCodeConfig struct {
	Size                 int    `json:"size" yaml:"size"`
	ErrorCorrectionLevel string `json:"errorCorrectionLevel" yaml:"errorCorrectionLevel"`
	BaseURL              string `json:"baseUrl" yaml:"baseUrl"`
}

// RoutingConfig defines routing engine configuration
type RoutingConfig struct {
	// Maximum distance in kilometers for GPS coordinate snapping to road network
	MaxSnapDistanceKm float64 `json:"maxSnapDistanceKm" yaml:"maxSnapDistanceKm"`

	// Default vehicle speed in km/h for duration estimation when routing data is unavailable
	DefaultSpeedKmh float64 `json:"defaultSpeedKmh" yaml:"defaultSpeedKmh"`

	// Path to routing data directory containing CH graph files
	DataPath string `json:"dataPath" yaml:"dataPath"`

	// Enable routing engine (set to false to use Haversine fallback only)
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Maximum query radius in kilometers for One-to-Many queries
	MaxQueryRadiusKm float64 `json:"maxQueryRadiusKm" yaml:"maxQueryRadiusKm"`

	// Number of concurrent workers for One-to-Many queries
	OneToManyWorkers int `json:"oneToManyWorkers" yaml:"oneToManyWorkers"`

	// Haversine pre-filter radius multiplier (e.g., 1.3 = filter targets beyond 1.3x max radius)
	PreFilterRadiusMultiplier float64 `json:"preFilterRadiusMultiplier" yaml:"preFilterRadiusMultiplier"`

	// Grid cell size in kilometers for spatial index
	GridCellSizeKm float64 `json:"gridCellSizeKm" yaml:"gridCellSizeKm"`
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

// PMTilesConfig defines PMTiles routing configuration
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
		return nil, errors.Errorf("config file %s.yaml not found in any search path", currEnv)
	}

	// Load YAML config file
	if err := koanfInstance.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		return nil, errors.Wrapf(err, "read %s config failed", currEnv)
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
		return nil, errors.Wrap(err, "load env variables failed")
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
		return nil, errors.Wrapf(err, "unmarshal %s config failed", currEnv)
	}

	return cfg, nil
}

func New() (*Config, error) {
	cfg, err := LoadWithEnv[Config]("config", "config", "../config", "../../config")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(cfg.HTTP.MaxRequestBodySize) == "" {
		cfg.HTTP.MaxRequestBodySize = defaultMaxRequestBodySize
	}

	// Build replicas from environment variables (POSTGRES_REPLICAS_0_HOST, POSTGRES_REPLICAS_0_PORT, etc.)
	cfg.Postgres.Replicas = buildReplicasFromEnv()

	return cfg, nil
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
