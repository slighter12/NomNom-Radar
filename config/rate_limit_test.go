package config

import (
	"math"
	"os"
	"strings"
	"testing"
	"time"
)

func TestApplyDefaults_RateLimit(t *testing.T) {
	cfg := &Config{}

	ApplyDefaults(cfg)

	if cfg.HTTP.RateLimit == nil {
		t.Fatal("expected rate limit defaults")
	}
	if !cfg.HTTP.RateLimit.IsEnabled() {
		t.Fatal("expected rate limit to be enabled by default")
	}
	if cfg.HTTP.RateLimit.Rate == nil || *cfg.HTTP.RateLimit.Rate != 10 {
		t.Fatalf("Rate = %v, want 10", cfg.HTTP.RateLimit.Rate)
	}
	if cfg.HTTP.RateLimit.Burst == nil || *cfg.HTTP.RateLimit.Burst != 30 {
		t.Fatalf("Burst = %v, want 30", cfg.HTTP.RateLimit.Burst)
	}
	if cfg.HTTP.RateLimit.ExpiresIn == nil || *cfg.HTTP.RateLimit.ExpiresIn != 3*time.Minute {
		t.Fatalf("ExpiresIn = %v, want 3m", cfg.HTTP.RateLimit.ExpiresIn)
	}
}

func TestApplyDefaults_APIRateLimit(t *testing.T) {
	cfg := &Config{}

	ApplyDefaults(cfg)

	if cfg.HTTP.APIRateLimit == nil {
		t.Fatal("expected API rate limit defaults")
	}
	if !cfg.HTTP.APIRateLimit.IsEnabled() {
		t.Fatal("expected API rate limit to be enabled by default")
	}
	if cfg.HTTP.APIRateLimit.Rate == nil || *cfg.HTTP.APIRateLimit.Rate != 20 {
		t.Fatalf("Rate = %v, want 20", cfg.HTTP.APIRateLimit.Rate)
	}
	if cfg.HTTP.APIRateLimit.Burst == nil || *cfg.HTTP.APIRateLimit.Burst != 60 {
		t.Fatalf("Burst = %v, want 60", cfg.HTTP.APIRateLimit.Burst)
	}
	if cfg.HTTP.APIRateLimit.ExpiresIn == nil || *cfg.HTTP.APIRateLimit.ExpiresIn != 3*time.Minute {
		t.Fatalf("ExpiresIn = %v, want 3m", cfg.HTTP.APIRateLimit.ExpiresIn)
	}
}

func TestApplyDefaults_SessionRateLimit(t *testing.T) {
	cfg := &Config{}

	ApplyDefaults(cfg)

	if cfg.HTTP.SessionRateLimit == nil {
		t.Fatal("expected session rate limit defaults")
	}
	if !cfg.HTTP.SessionRateLimit.IsEnabled() {
		t.Fatal("expected session rate limit to be enabled by default")
	}
	if cfg.HTTP.SessionRateLimit.Rate == nil || *cfg.HTTP.SessionRateLimit.Rate != 2 {
		t.Fatalf("Rate = %v, want 2", cfg.HTTP.SessionRateLimit.Rate)
	}
	if cfg.HTTP.SessionRateLimit.Burst == nil || *cfg.HTTP.SessionRateLimit.Burst != 20 {
		t.Fatalf("Burst = %v, want 20", cfg.HTTP.SessionRateLimit.Burst)
	}
	if cfg.HTTP.SessionRateLimit.ExpiresIn == nil || *cfg.HTTP.SessionRateLimit.ExpiresIn != 3*time.Minute {
		t.Fatalf("ExpiresIn = %v, want 3m", cfg.HTTP.SessionRateLimit.ExpiresIn)
	}
}

func TestApplyDefaults_RateLimitPreservesDisabledValue(t *testing.T) {
	enabled := false
	cfg := &Config{}
	cfg.HTTP.RateLimit = &RateLimitConfig{Enabled: &enabled}

	ApplyDefaults(cfg)

	if cfg.HTTP.RateLimit.IsEnabled() {
		t.Fatal("expected explicit disabled value to be preserved")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyDefaults_APIRateLimitPreservesDisabledValue(t *testing.T) {
	enabled := false
	cfg := &Config{}
	cfg.HTTP.APIRateLimit = &RateLimitConfig{Enabled: &enabled}

	ApplyDefaults(cfg)

	if cfg.HTTP.APIRateLimit.IsEnabled() {
		t.Fatal("expected explicit API rate limit disabled value to be preserved")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestConfigValidateRejectsInvalidAPIRateLimit(t *testing.T) {
	settings := newRateLimitConfig(0, 1, time.Minute)
	cfg := &Config{}
	cfg.HTTP.APIRateLimit = &settings

	ApplyDefaults(cfg)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected an error for invalid API rate-limit values")
	}
	if !strings.Contains(err.Error(), "http.apiRateLimit.rate") {
		t.Fatalf("Validate() error = %v, want API rate-limit path", err)
	}
	if strings.Contains(err.Error(), "http.rateLimit") {
		t.Fatalf("Validate() error = %v, must not use authentication rate-limit path", err)
	}
}

func TestConfigValidateRejectsInvalidSessionRateLimit(t *testing.T) {
	settings := newRateLimitConfig(0, 1, time.Minute)
	cfg := &Config{}
	cfg.HTTP.SessionRateLimit = &settings

	ApplyDefaults(cfg)

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() expected an error for invalid session rate-limit values")
	}
	if !strings.Contains(err.Error(), "http.sessionRateLimit.rate") {
		t.Fatalf("Validate() error = %v, want session rate-limit path", err)
	}
	if strings.Contains(err.Error(), "http.rateLimit") {
		t.Fatalf("Validate() error = %v, must not use authentication rate-limit path", err)
	}
}

func TestRateLimitConfigValidateUsesDefaultAndExplicitPaths(t *testing.T) {
	settings := newRateLimitConfig(0, 1, time.Minute)

	err := settings.ValidateAt("http.rateLimit")
	if err == nil || !strings.Contains(err.Error(), "http.rateLimit.rate") {
		t.Fatalf("ValidateAt() error = %v, want authentication rate-limit path", err)
	}

	err = settings.ValidateAt("http.apiRateLimit")
	if err == nil || !strings.Contains(err.Error(), "http.apiRateLimit.rate") {
		t.Fatalf("ValidateAt() error = %v, want API rate-limit path", err)
	}
}

func TestApplyDefaults_RateLimitPreservesExplicitZeroValues(t *testing.T) {
	rate := 0.0
	burst := 0
	expiresIn := time.Duration(0)
	cfg := &Config{}
	cfg.HTTP.RateLimit = &RateLimitConfig{
		Rate:      &rate,
		Burst:     &burst,
		ExpiresIn: &expiresIn,
	}

	ApplyDefaults(cfg)

	if *cfg.HTTP.RateLimit.Rate != 0 || *cfg.HTTP.RateLimit.Burst != 0 || *cfg.HTTP.RateLimit.ExpiresIn != 0 {
		t.Fatalf("ApplyDefaults overwrote explicit zero values: %+v", cfg.HTTP.RateLimit)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected an error for explicit zero rate-limit values")
	}
}

func TestRateLimitConfigValidateRejectsInvalidEnabledValues(t *testing.T) {
	testCases := []RateLimitConfig{
		newRateLimitConfig(0, 1, time.Minute),
		newRateLimitConfig(1, 0, time.Minute),
		newRateLimitConfig(1, 1, 0),
		newRateLimitConfig(-1, 1, time.Minute),
	}

	for _, cfg := range testCases {
		if err := cfg.ValidateAt("http.rateLimit"); err == nil {
			t.Fatalf("Validate(%+v) expected an error", cfg)
		}
	}
}

func TestRateLimitConfigValidateRejectsNonFiniteRate(t *testing.T) {
	testCases := []struct {
		name string
		rate float64
	}{
		{name: "NaN", rate: math.NaN()},
		{name: "positive infinity", rate: math.Inf(1)},
		{name: "negative infinity", rate: math.Inf(-1)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := newRateLimitConfig(testCase.rate, 1, time.Minute)
			if err := cfg.ValidateAt("http.rateLimit"); err == nil {
				t.Fatalf("Validate(%+v) expected an error", cfg)
			}
		})
	}
}

func TestLoadWithEnv_RateLimitPreservesExplicitZeroValues(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("test.yaml", []byte("http:\n  rateLimit:\n    enabled: true\n    rate: 10\n    burst: 30\n    expiresIn: 3m\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("HTTP_RATELIMIT_RATE", "0")
	t.Setenv("HTTP_RATELIMIT_BURST", "0")
	t.Setenv("HTTP_RATELIMIT_EXPIRESIN", "0s")

	cfg, err := LoadWithEnv[Config]("test")
	if err != nil {
		t.Fatalf("LoadWithEnv() error = %v", err)
	}
	ApplyDefaults(cfg)

	if *cfg.HTTP.RateLimit.Rate != 0 || *cfg.HTTP.RateLimit.Burst != 0 || *cfg.HTTP.RateLimit.ExpiresIn != 0 {
		t.Fatalf("loaded explicit zero values were not preserved: %+v", cfg.HTTP.RateLimit)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() expected an error for loaded explicit zero values")
	}
}

func newRateLimitConfig(rate float64, burst int, expiresIn time.Duration) RateLimitConfig {
	return RateLimitConfig{
		Rate:      &rate,
		Burst:     &burst,
		ExpiresIn: &expiresIn,
	}
}
