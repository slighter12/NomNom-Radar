package auth

import (
	"context"
	"strings"
	"testing"

	"radar/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArgon2idHasher_RoundTrip(t *testing.T) {
	hasher := newTestArgon2idHasher(t)

	testCases := []struct {
		name     string
		password string
	}{
		{name: "ascii", password: "StrongPass123!"},
		{name: "unicode", password: "Pässphräse123!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			hash, err := hasher.Hash(tc.password)
			require.NoError(t, err)
			assert.NotEmpty(t, hash)
			assert.True(t, hasher.Check(tc.password, hash))
		})
	}
}

func TestArgon2idHasher_WrongPasswordAndTamperedHash(t *testing.T) {
	hasher := newTestArgon2idHasher(t)
	password := "StrongPass123!"

	hash, err := hasher.Hash(password)
	require.NoError(t, err)

	testCases := []struct {
		name         string
		password     string
		encodedHash  string
		expectedOkay bool
	}{
		{
			name:         "wrong password",
			password:     "WrongPass123!",
			encodedHash:  hash,
			expectedOkay: false,
		},
		{
			name:         "tampered hash",
			password:     password,
			encodedHash:  hash[:len(hash)-1] + "A",
			expectedOkay: false,
		},
		{
			name:         "format mismatch",
			password:     password,
			encodedHash:  "not-a-valid-hash",
			expectedOkay: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ok, verifyErr := hasher.VerifyWithContext(context.Background(), tc.password, tc.encodedHash)
			require.NoError(t, verifyErr)
			assert.Equal(t, tc.expectedOkay, ok)
		})
	}
}

func TestArgon2idHasher_ContextCancellation(t *testing.T) {
	hasher := newTestArgon2idHasher(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := hasher.HashWithContext(ctx, "StrongPass123!")
	require.ErrorIs(t, err, context.Canceled)

	hash, err := hasher.Hash("StrongPass123!")
	require.NoError(t, err)

	ok, verifyErr := hasher.VerifyWithContext(ctx, "StrongPass123!", hash)
	require.ErrorIs(t, verifyErr, context.Canceled)
	assert.False(t, ok)
}

func TestArgon2idHasher_RandomSaltUniqueness(t *testing.T) {
	hasher := newTestArgon2idHasher(t)
	password := "StrongPass123!"

	hash1, err := hasher.Hash(password)
	require.NoError(t, err)

	hash2, err := hasher.Hash(password)
	require.NoError(t, err)

	assert.NotEqual(t, hash1, hash2)
	assert.True(t, strings.HasPrefix(hash1, "$argon2id$v=19$"))
	assert.True(t, strings.HasPrefix(hash2, "$argon2id$v=19$"))
}

func TestArgon2idHasher_ValidatePasswordStrength(t *testing.T) {
	hasher := newTestArgon2idHasher(t)

	err := hasher.ValidatePasswordStrength("weak")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must be at least 8 characters long")
}

func newTestArgon2idHasher(t *testing.T) *Argon2idHasher {
	t.Helper()

	cfg := &config.Config{
		Auth: &config.AuthConfig{
			Argon2Memory:        64,
			Argon2Iterations:    1,
			Argon2Parallelism:   1,
			Argon2MaxConcurrent: 2,
		},
		PasswordStrength: &config.PasswordStrengthConfig{
			MinLength:        8,
			RequireUppercase: true,
			RequireLowercase: true,
			RequireNumbers:   true,
			RequireSpecial:   true,
			MaxLength:        128,
		},
	}

	hasher, err := NewArgon2idHasher(cfg)
	require.NoError(t, err)

	concreteHasher, ok := hasher.(*Argon2idHasher)
	require.True(t, ok)

	return concreteHasher
}
