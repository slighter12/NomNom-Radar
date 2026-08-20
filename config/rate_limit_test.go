package config

import (
	"math"
	"os"
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
		if err := cfg.Validate(); err == nil {
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
			if err := cfg.Validate(); err == nil {
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
