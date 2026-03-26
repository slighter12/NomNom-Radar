package qrcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"radar/config"

	"github.com/google/uuid"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestConfig(level string) *config.Config {
	return &config.Config{
		QRCode: &config.QRCodeConfig{
			ErrorCorrectionLevel: level,
		},
	}
}
func TestNewQRCodeService(t *testing.T) {
	tests := []struct {
		name                 string
		errorCorrectionLevel string
	}{
		{"Low error correction", "L"},
		{"Medium error correction", "M"},
		{"High error correction", "Q"},
		{"Highest error correction", "H"},
		{"Default error correction", "invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := newTestConfig(tt.errorCorrectionLevel)
			service := NewQRCodeService(cfg)
			assert.NotNil(t, service)
		})
	}
}

func TestQRCodeService_GenerateSubscriptionQR(t *testing.T) {
	cfg := newTestConfig("M")
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

func TestQRCodeService_GenerateSubscriptionQR_IsDecodable(t *testing.T) {
	service := NewQRCodeService(newTestConfig("M"))
	merchantID := uuid.New()

	qrBytes, err := service.GenerateSubscriptionQR(merchantID)
	require.NoError(t, err)
	assert.NotEmpty(t, qrBytes)

	decodedPayload, err := decodeQRCodePayload(qrBytes)
	require.NoError(t, err)
	assert.Equal(t, expectedSubscriptionPayload(t, merchantID), decodedPayload)
}

func TestQRCodeService_GenerateSubscriptionQR_ErrorCorrectionLevels(t *testing.T) {
	levels := []string{"L", "M", "Q", "H"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			service := NewQRCodeService(newTestConfig(level))
			merchantID := uuid.New()

			qrBytes, err := service.GenerateSubscriptionQR(merchantID)
			require.NoError(t, err)
			assert.NotEmpty(t, qrBytes)

			decodedPayload, err := decodeQRCodePayload(qrBytes)
			require.NoError(t, err)
			assert.Equal(t, expectedSubscriptionPayload(t, merchantID), decodedPayload)
		})
	}
}

func TestQRCodeService_ParseSubscriptionQR(t *testing.T) {
	cfg := newTestConfig("M")
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
	cfg := newTestConfig("M")
	service := NewQRCodeService(cfg)

	_, err := service.ParseSubscriptionQR("invalid json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid character")
}

func TestQRCodeService_ParseSubscriptionQR_InvalidType(t *testing.T) {
	cfg := newTestConfig("M")
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
	cfg := newTestConfig("M")
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
	cfg := newTestConfig("M")
	service := NewQRCodeService(cfg)
	originalMerchantID := uuid.New()

	// Generate QR code
	qrBytes, err := service.GenerateSubscriptionQR(originalMerchantID)
	require.NoError(t, err)
	assert.NotEmpty(t, qrBytes)

	decodedPayload, err := decodeQRCodePayload(qrBytes)
	require.NoError(t, err)

	parsedID, err := service.ParseSubscriptionQR(decodedPayload)
	require.NoError(t, err)
	assert.Equal(t, originalMerchantID, parsedID)
}

func decodeQRCodePayload(qrBytes []byte) (string, error) {
	img, err := png.Decode(bytes.NewReader(qrBytes))
	if err != nil {
		return "", fmt.Errorf("decode PNG payload: %w", err)
	}

	result, err := decodeQRImage(img)
	if err != nil {
		return "", err
	}

	return result.GetText(), nil
}

func decodeQRImage(img image.Image) (*gozxing.Result, error) {
	tunedHints := map[gozxing.DecodeHintType]any{
		gozxing.DecodeHintType_TRY_HARDER: true,
	}
	pureHints := map[gozxing.DecodeHintType]any{
		gozxing.DecodeHintType_TRY_HARDER:   true,
		gozxing.DecodeHintType_PURE_BARCODE: true,
	}
	var lastErr error
	thresholdedImg := thresholdQRCodeImage(img)

	for _, tc := range []struct {
		source image.Image
		hints  map[gozxing.DecodeHintType]any
	}{
		{source: img, hints: nil},
		{source: img, hints: tunedHints},
		{source: thresholdedImg, hints: tunedHints},
		{source: thresholdedImg, hints: pureHints},
	} {
		bitmap, err := gozxing.NewBinaryBitmapFromImage(tc.source)
		if err != nil {
			lastErr = fmt.Errorf("build binary bitmap: %w", err)

			continue
		}

		result, err := qrcode.NewQRCodeReader().Decode(bitmap, tc.hints)
		if err == nil {
			return result, nil
		}

		lastErr = err
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("no decode attempts were executed")
	}

	return nil, fmt.Errorf("failed to decode QR image: %w", lastErr)
}

func thresholdQRCodeImage(img image.Image) image.Image {
	bounds := img.Bounds()
	dst := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			luminance := (299*r + 587*g + 114*b) / 1000
			if luminance >= 0x8000 {
				dst.SetGray(x, y, color.Gray{Y: 0xFF})

				continue
			}

			dst.SetGray(x, y, color.Gray{Y: 0x00})
		}
	}

	return dst
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
