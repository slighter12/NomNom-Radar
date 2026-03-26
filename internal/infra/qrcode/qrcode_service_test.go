package qrcode

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"testing"

	"radar/config"

	"github.com/google/uuid"
	"github.com/makiuchi-d/gozxing"
	gozxingqrcode "github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfig(size int, level string) *config.Config {
	return &config.Config{
		QRCode: &config.QRCodeConfig{
			Size:                 size,
			ErrorCorrectionLevel: level,
		},
	}
}
func TestNewQRCodeService(t *testing.T) {
	tests := []struct {
		name                 string
		size                 int
		errorCorrectionLevel string
	}{
		{"Low error correction", 256, "L"},
		{"Medium error correction", 256, "M"},
		{"High error correction", 256, "Q"},
		{"Highest error correction", 256, "H"},
		{"Default error correction", 256, "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig(tt.size, tt.errorCorrectionLevel)
			service := NewQRCodeService(cfg)
			assert.NotNil(t, service)
		})
	}
}

func TestQRCodeService_GenerateSubscriptionQR(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)
	merchantID := uuid.New()

	qrBytes, err := service.GenerateSubscriptionQR(merchantID)
	require.NoError(t, err)
	assert.NotEmpty(t, qrBytes)

	// Verify it's a valid PNG (starts with PNG magic number)
	assert.Equal(t, byte(0x89), qrBytes[0])
	assert.Equal(t, byte(0x50), qrBytes[1])
	assert.Equal(t, byte(0x4E), qrBytes[2])
	assert.Equal(t, byte(0x47), qrBytes[3])
}

func TestQRCodeService_GenerateSubscriptionQR_DifferentSizes(t *testing.T) {
	tests := []struct {
		name string
		size int
	}{
		{"Small QR", 128},
		{"Medium QR", 256},
		{"Large QR", 512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig(tt.size, "M")
			service := NewQRCodeService(cfg)
			merchantID := uuid.New()

			qrBytes, err := service.GenerateSubscriptionQR(merchantID)
			require.NoError(t, err)
			assert.NotEmpty(t, qrBytes)

			imgCfg, err := png.DecodeConfig(bytes.NewReader(qrBytes))
			require.NoError(t, err)
			assert.Equal(t, tt.size, imgCfg.Width)
			assert.Equal(t, tt.size, imgCfg.Height)

			decodedPayload := decodeQRCodePayload(t, qrBytes)
			assert.Equal(t, expectedSubscriptionPayload(t, merchantID), decodedPayload)
		})
	}
}

func TestQRCodeService_GenerateSubscriptionQR_ErrorCorrectionLevels(t *testing.T) {
	levels := []string{"L", "M", "Q", "H"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			service := NewQRCodeService(newTestConfig(256, level))
			merchantID := uuid.New()

			qrBytes, err := service.GenerateSubscriptionQR(merchantID)
			require.NoError(t, err)
			assert.NotEmpty(t, qrBytes)
			assert.Equal(t, expectedSubscriptionPayload(t, merchantID), decodeQRCodePayload(t, qrBytes))
		})
	}
}

func TestQRCodeService_ParseSubscriptionQR(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)
	merchantID := uuid.New()

	// Create valid QR data
	data := QRCodeData{
		MerchantID: merchantID.String(),
		Type:       "subscription",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	// Parse the QR data
	parsedID, err := service.ParseSubscriptionQR(string(jsonData))
	require.NoError(t, err)
	assert.Equal(t, merchantID, parsedID)
}

func TestQRCodeService_ParseSubscriptionQR_InvalidJSON(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)

	_, err := service.ParseSubscriptionQR("invalid json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestQRCodeService_ParseSubscriptionQR_InvalidType(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)

	// Create QR data with invalid type
	data := QRCodeData{
		MerchantID: uuid.New().String(),
		Type:       "invalid_type",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	_, err = service.ParseSubscriptionQR(string(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid QR code type")
}

func TestQRCodeService_ParseSubscriptionQR_InvalidUUID(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)

	// Create QR data with invalid UUID
	data := QRCodeData{
		MerchantID: "not-a-valid-uuid",
		Type:       "subscription",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	_, err = service.ParseSubscriptionQR(string(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid UUID")
}

func TestQRCodeService_RoundTrip(t *testing.T) {
	cfg := newTestConfig(256, "M")
	service := NewQRCodeService(cfg)
	originalMerchantID := uuid.New()

	// Generate QR code
	qrBytes, err := service.GenerateSubscriptionQR(originalMerchantID)
	require.NoError(t, err)
	assert.NotEmpty(t, qrBytes)

	decodedPayload := decodeQRCodePayload(t, qrBytes)

	parsedID, err := service.ParseSubscriptionQR(decodedPayload)
	require.NoError(t, err)
	assert.Equal(t, originalMerchantID, parsedID)
}

func decodeQRCodePayload(t *testing.T, qrBytes []byte) string {
	t.Helper()

	img, err := png.Decode(bytes.NewReader(qrBytes))
	require.NoError(t, err)

	result := decodeQRImage(t, img)

	return result.GetText()
}

func decodeQRImage(t *testing.T, img image.Image) *gozxing.Result {
	t.Helper()

	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	require.NoError(t, err)

	result, err := gozxingqrcode.NewQRCodeReader().Decode(bitmap, nil)
	require.NoError(t, err)

	return result
}

func expectedSubscriptionPayload(t *testing.T, merchantID uuid.UUID) string {
	t.Helper()

	data := QRCodeData{
		MerchantID: merchantID.String(),
		Type:       "subscription",
	}

	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	return string(jsonData)
}
