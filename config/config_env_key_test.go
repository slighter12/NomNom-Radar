package config

import "testing"

func TestCanonicalizeEnvKey_UsesExistingCamelCaseKeys(t *testing.T) {
	existing := map[string]any{
		"postgres": map[string]any{
			"sslMode": "disable",
			"master": map[string]any{
				"userName": "user",
			},
		},
		"pubsub": map[string]any{
			"topicId": "",
		},
		"secretKey": map[string]any{
			"access": "",
		},
	}

	tests := []struct {
		envKey string
		want   string
	}{
		{envKey: "POSTGRES_SSLMODE", want: "postgres.sslMode"},
		{envKey: "POSTGRES_MASTER_USERNAME", want: "postgres.master.userName"},
		{envKey: "PUBSUB_TOPICID", want: "pubsub.topicId"},
		{envKey: "SECRETKEY_ACCESS", want: "secretKey.access"},
		{envKey: "NEW_FEATURE_FLAG", want: "new.feature.flag"},
	}

	for _, tt := range tests {
		t.Run(tt.envKey, func(t *testing.T) {
			if got := canonicalizeEnvKey(tt.envKey, existing); got != tt.want {
				t.Fatalf("canonicalizeEnvKey(%q) = %q, want %q", tt.envKey, got, tt.want)
			}
		})
	}
}
