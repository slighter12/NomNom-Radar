// Package auth provides concrete implementations for authentication-related domain services.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"radar/config"
	"radar/internal/domain/service"

	"golang.org/x/crypto/argon2"
)

const (
	defaultArgon2Memory        uint32 = 65536
	defaultArgon2Iterations    uint32 = 3
	defaultArgon2Parallelism   uint8  = 1
	defaultArgon2MaxConcurrent int    = 4
	argon2Version                     = 19
	argon2SaltLength                  = 16
	argon2KeyLength            uint32 = 32
	passwordSpecialChars              = "!@#$%^&*()_+-=[]{};':\"\\|,.<>/?~`"
)

// Argon2idHasher is a concrete implementation of the PasswordHasher interface using Argon2id.
type Argon2idHasher struct {
	memory                 uint32
	iterations             uint32
	parallelism            uint8
	semaphore              chan struct{}
	passwordStrengthConfig *config.PasswordStrengthConfig
}

// NewArgon2idHasher creates an Argon2id hasher with custom configuration.
func NewArgon2idHasher(cfg *config.Config) (service.PasswordHasher, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if cfg.PasswordStrength == nil {
		return nil, errors.New("passwordStrength config is required")
	}

	memory := defaultArgon2Memory
	iterations := defaultArgon2Iterations
	parallelism := defaultArgon2Parallelism
	maxConcurrent := defaultArgon2MaxConcurrent

	if cfg.Auth != nil {
		if cfg.Auth.Argon2Memory > 0 {
			memory = cfg.Auth.Argon2Memory
		}
		if cfg.Auth.Argon2Iterations > 0 {
			iterations = cfg.Auth.Argon2Iterations
		}
		if cfg.Auth.Argon2Parallelism > 0 {
			parallelism = cfg.Auth.Argon2Parallelism
		}
		if cfg.Auth.Argon2MaxConcurrent > 0 {
			maxConcurrent = cfg.Auth.Argon2MaxConcurrent
		}
	}

	return &Argon2idHasher{
		memory:                 memory,
		iterations:             iterations,
		parallelism:            parallelism,
		semaphore:              make(chan struct{}, maxConcurrent),
		passwordStrengthConfig: cfg.PasswordStrength,
	}, nil
}

// Hash generates a salted hash from a plaintext password using Argon2id.
func (h *Argon2idHasher) Hash(password string) (string, error) {
	return h.HashWithContext(context.Background(), password)
}

// HashWithContext generates a salted hash from a plaintext password using Argon2id while observing cancellation.
func (h *Argon2idHasher) HashWithContext(ctx context.Context, password string) (string, error) {
	if err := h.ValidatePasswordStrength(password); err != nil {
		return "", err
	}

	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()

	salt := make([]byte, argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(password), salt, h.iterations, h.memory, h.parallelism, argon2KeyLength)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2Version,
		h.memory,
		h.iterations,
		h.parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Check compares a plaintext password with an Argon2id hash.
func (h *Argon2idHasher) Check(password, hash string) bool {
	ok, err := h.VerifyWithContext(context.Background(), password, hash)

	return err == nil && ok
}

// VerifyWithContext compares a plaintext password with an Argon2id hash while observing cancellation.
func (h *Argon2idHasher) VerifyWithContext(ctx context.Context, password, encodedHash string) (bool, error) {
	parsedHash, err := parseArgon2idHash(encodedHash)
	if err != nil {
		return false, nil
	}

	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()

	computedHash := argon2.IDKey(
		[]byte(password),
		parsedHash.salt,
		parsedHash.iterations,
		parsedHash.memory,
		parsedHash.parallelism,
		uint32(len(parsedHash.hash)),
	)

	return subtle.ConstantTimeCompare(computedHash, parsedHash.hash) == 1, nil
}

// ValidatePasswordStrength validates that a password meets security requirements.
func (h *Argon2idHasher) ValidatePasswordStrength(password string) error {
	// Validate all password requirements
	if err := h.validatePasswordLength(password, h.passwordStrengthConfig.MinLength, h.passwordStrengthConfig.MaxLength); err != nil {
		return err
	}

	if err := h.validatePasswordCharacters(password); err != nil {
		return err
	}

	return nil
}

// validatePasswordLength checks if password meets length requirements
func (h *Argon2idHasher) validatePasswordLength(password string, minLength, maxLength int) error {
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters long", minLength)
	}

	if maxLength > 0 && len(password) > maxLength {
		return fmt.Errorf("password must be no more than %d characters long", maxLength)
	}

	return nil
}

// validatePasswordCharacters checks if password contains required character types
func (h *Argon2idHasher) validatePasswordCharacters(password string) error {
	flags := passwordCharacterFlags{
		hasUppercase: !h.passwordStrengthConfig.RequireUppercase,
		hasLowercase: !h.passwordStrengthConfig.RequireLowercase,
		hasNumbers:   !h.passwordStrengthConfig.RequireNumbers,
		hasSpecial:   !h.passwordStrengthConfig.RequireSpecial,
	}

	for _, char := range password {
		flags.mark(char)
		if flags.complete() {
			break
		}
	}

	if !flags.hasUppercase {
		return errors.New("password must contain at least one uppercase letter")
	}

	if !flags.hasLowercase {
		return errors.New("password must contain at least one lowercase letter")
	}

	if !flags.hasNumbers {
		return errors.New("password must contain at least one number")
	}

	if !flags.hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

type passwordCharacterFlags struct {
	hasUppercase bool
	hasLowercase bool
	hasNumbers   bool
	hasSpecial   bool
}

func (f *passwordCharacterFlags) mark(char rune) {
	switch {
	case unicode.IsUpper(char):
		f.hasUppercase = true
	case unicode.IsLower(char):
		f.hasLowercase = true
	case unicode.IsDigit(char):
		f.hasNumbers = true
	case strings.ContainsRune(passwordSpecialChars, char):
		f.hasSpecial = true
	}
}

func (f passwordCharacterFlags) complete() bool {
	return f.hasUppercase && f.hasLowercase && f.hasNumbers && f.hasSpecial
}

type parsedArgon2idHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	hash        []byte
}

func parseArgon2idHash(encodedHash string) (*parsedArgon2idHash, error) {
	trimmed := strings.TrimPrefix(encodedHash, "$")
	parts := strings.Split(trimmed, "$")
	if len(parts) != 5 {
		return nil, errors.New("invalid hash format")
	}

	if parts[0] != "argon2id" {
		return nil, errors.New("invalid hash algorithm")
	}

	version, err := parseVersion(parts[1])
	if err != nil {
		return nil, err
	}
	if version != argon2Version {
		return nil, errors.New("invalid hash version")
	}

	memory, iterations, parallelism, err := parseParameters(parts[2])
	if err != nil {
		return nil, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, errors.New("invalid salt encoding")
	}

	hash, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, errors.New("invalid hash encoding")
	}

	return &parsedArgon2idHash{
		memory:      memory,
		iterations:  iterations,
		parallelism: parallelism,
		salt:        salt,
		hash:        hash,
	}, nil
}

func parseVersion(raw string) (int, error) {
	value, found := strings.CutPrefix(raw, "v=")
	if !found {
		return 0, errors.New("invalid version format")
	}

	version, err := strconv.Atoi(value)
	if err != nil {
		return 0, errors.New("invalid version value")
	}

	return version, nil
}

func parseParameters(raw string) (uint32, uint32, uint8, error) {
	values := strings.Split(raw, ",")
	if len(values) != 3 {
		return 0, 0, 0, errors.New("invalid parameter format")
	}

	var (
		memory      uint64
		iterations  uint64
		parallelism uint64
		err         error
	)

	for _, value := range values {
		switch {
		case strings.HasPrefix(value, "m="):
			memory, err = strconv.ParseUint(strings.TrimPrefix(value, "m="), 10, 32)
		case strings.HasPrefix(value, "t="):
			iterations, err = strconv.ParseUint(strings.TrimPrefix(value, "t="), 10, 32)
		case strings.HasPrefix(value, "p="):
			parallelism, err = strconv.ParseUint(strings.TrimPrefix(value, "p="), 10, 8)
		default:
			return 0, 0, 0, errors.New("invalid parameter key")
		}
		if err != nil {
			return 0, 0, 0, errors.New("invalid parameter value")
		}
	}

	return uint32(memory), uint32(iterations), uint8(parallelism), nil
}

func (h *Argon2idHasher) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("acquire hasher semaphore pre-check: %w", ctx.Err())
	default:
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("acquire hasher semaphore: %w", ctx.Err())
	case h.semaphore <- struct{}{}:
		return nil
	}
}

func (h *Argon2idHasher) release() {
	<-h.semaphore
}
