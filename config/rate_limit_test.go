package config

import (
	"math"
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
	if cfg.HTTP.RateLimit.Rate != 10 {
		t.Fatalf("Rate = %v, want 10", cfg.HTTP.RateLimit.Rate)
	}
	if cfg.HTTP.RateLimit.Burst != 30 {
		t.Fatalf("Burst = %d, want 30", cfg.HTTP.RateLimit.Burst)
	}
	if cfg.HTTP.RateLimit.ExpiresIn != 3*time.Minute {
		t.Fatalf("ExpiresIn = %s, want 3m", cfg.HTTP.RateLimit.ExpiresIn)
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

func TestRateLimitConfigValidateRejectsInvalidEnabledValues(t *testing.T) {
	testCases := []RateLimitConfig{
		{Rate: 0, Burst: 1, ExpiresIn: time.Minute},
		{Rate: 1, Burst: 0, ExpiresIn: time.Minute},
		{Rate: 1, Burst: 1, ExpiresIn: 0},
		{Rate: -1, Burst: 1, ExpiresIn: time.Minute},
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
			cfg := RateLimitConfig{Rate: testCase.rate, Burst: 1, ExpiresIn: time.Minute}
			if err := cfg.Validate(); err == nil {
				t.Fatalf("Validate(%+v) expected an error", cfg)
			}
		})
	}
}
