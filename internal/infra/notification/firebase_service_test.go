package notification

import (
	"context"
	"log/slog"
	"testing"

	"radar/config"
)

func TestNewFirebaseService_WithLocalDemoConfig_UsesNoopService(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env.Env = "local"
	cfg.Firebase = &config.FirebaseConfig{
		ProjectID:       "demo-project-id",
		CredentialsPath: "/path/to/demo-firebase-service-account.json",
	}

	notificationSvc, err := NewFirebaseService(FirebaseDependencies{
		LC:     context.Background(),
		Config: cfg,
		Logger: slog.Default(),
	})
	if err != nil {
		t.Fatalf("NewFirebaseService returned error: %v", err)
	}

	successCount, failureCount, invalidTokens, err := notificationSvc.SendBatchNotification(
		context.Background(),
		[]string{"token-a", "token-b"},
		"title",
		"body",
		nil,
	)
	if err != nil {
		t.Fatalf("SendBatchNotification returned error: %v", err)
	}
	if successCount != 2 || failureCount != 0 || len(invalidTokens) != 0 {
		t.Fatalf(
			"unexpected noop result: success=%d failure=%d invalid=%d",
			successCount,
			failureCount,
			len(invalidTokens),
		)
	}
}

func TestNewFirebaseService_WithLocalRealConfig_RequiresCredentialsFile(t *testing.T) {
	cfg := &config.Config{}
	cfg.Env.Env = "local"
	cfg.Firebase = &config.FirebaseConfig{
		ProjectID:       "real-project",
		CredentialsPath: "/path/to/missing-real-firebase-service-account.json",
	}

	_, err := NewFirebaseService(FirebaseDependencies{
		LC:     context.Background(),
		Config: cfg,
		Logger: slog.Default(),
	})
	if err == nil {
		t.Fatal("NewFirebaseService returned nil error for missing real credentials")
	}
}
