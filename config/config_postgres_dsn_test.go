package config

import (
	"strings"
	"testing"

	"github.com/slighter12/go-lib/database/postgres"
)

func TestApplyPostgresMasterDSNFromEnv_InvalidSupabaseDSN_ReturnsError(t *testing.T) {
	cfg := &Config{
		Postgres: &postgres.DBConn{
			Master: postgres.ConnectionConfig{
				Host:     "localhost",
				Port:     "5432",
				UserName: "user",
				Password: "password",
			},
			Database: "auth_db",
		},
	}

	// Mimic a placeholder DSN value copied directly from docs.
	t.Setenv(
		postgresMasterDSNEnvKey,
		"postgresql://postgres.fsiewohfvathirorxhau:[YOUR-PASSWORD]@aws-1-ap-northeast-1.pooler.supabase.com:6543/postgres?sslmode=require",
	)

	err := applyPostgresMasterDSNFromEnv(cfg)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}

	if !strings.Contains(err.Error(), "parse "+postgresMasterDSNEnvKey) {
		t.Fatalf("unexpected error: %v", err)
	}

	// Ensure configuration is unchanged on parse failure.
	if cfg.Postgres.Master.Host != "localhost" {
		t.Fatalf("host changed on parse failure: got %q", cfg.Postgres.Master.Host)
	}
	if cfg.Postgres.Database != "auth_db" {
		t.Fatalf("database changed on parse failure: got %q", cfg.Postgres.Database)
	}
}

func TestApplyPostgresMasterDSNFromEnv_ValidSupabaseDSN_OverridesMasterConfig(t *testing.T) {
	cfg := &Config{
		Postgres: &postgres.DBConn{
			Master: postgres.ConnectionConfig{
				Host:     "localhost",
				Port:     "5432",
				UserName: "user",
				Password: "password",
			},
			Database: "auth_db",
		},
	}

	// Password is URL-encoded as recommended for special characters.
	t.Setenv(
		postgresMasterDSNEnvKey,
		"postgresql://postgres.fsiewohfvathirorxhau:p%40ss%2Fword@aws-1-ap-northeast-1.pooler.supabase.com:6543/postgres?sslmode=require",
	)

	if err := applyPostgresMasterDSNFromEnv(cfg); err != nil {
		t.Fatalf("applyPostgresMasterDSNFromEnv returned error: %v", err)
	}

	if got := cfg.Postgres.Master.Host; got != "aws-1-ap-northeast-1.pooler.supabase.com" {
		t.Fatalf("unexpected host: got %q", got)
	}
	if got := cfg.Postgres.Master.Port; got != "6543" {
		t.Fatalf("unexpected port: got %q", got)
	}
	if got := cfg.Postgres.Master.UserName; got != "postgres.fsiewohfvathirorxhau" {
		t.Fatalf("unexpected username: got %q", got)
	}
	if got := cfg.Postgres.Master.Password; got != "p@ss/word" {
		t.Fatalf("unexpected password: got %q", got)
	}
	if got := cfg.Postgres.Database; got != "postgres" {
		t.Fatalf("unexpected database: got %q", got)
	}
}
