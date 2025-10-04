package qrcode

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
			service := NewQRCodeService(tt.size, tt.errorCorrectionLevel)
			assert.NotNil(t, service)
		})
	}
}

func TestQRCodeService_GenerateSubscriptionQR(t *testing.T) {
	service := NewQRCodeService(256, "M")
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
			service := NewQRCodeService(tt.size, "M")
			merchantID := uuid.New()

			qrBytes, err := service.GenerateSubscriptionQR(merchantID)
			require.NoError(t, err)
			assert.NotEmpty(t, qrBytes)
		})
	}
}

func TestQRCodeService_ParseSubscriptionQR(t *testing.T) {
	service := NewQRCodeService(256, "M")
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
	service := NewQRCodeService(256, "M")

	_, err := service.ParseSubscriptionQR("invalid json")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal QR code data")
}

func TestQRCodeService_ParseSubscriptionQR_InvalidType(t *testing.T) {
	service := NewQRCodeService(256, "M")

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
	service := NewQRCodeService(256, "M")

	// Create QR data with invalid UUID
	data := QRCodeData{
		MerchantID: "not-a-valid-uuid",
		Type:       "subscription",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	_, err = service.ParseSubscriptionQR(string(jsonData))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse merchant ID")
}

func TestQRCodeService_RoundTrip(t *testing.T) {
	service := NewQRCodeService(256, "M")
	originalMerchantID := uuid.New()

	// Generate QR code
	qrBytes, err := service.GenerateSubscriptionQR(originalMerchantID)
	require.NoError(t, err)
	assert.NotEmpty(t, qrBytes)

	// Note: We can't directly parse the PNG bytes back to JSON
	// In real usage, the QR code would be scanned by a device
	// and the JSON string would be extracted
	// For testing, we verify the data structure manually
	data := QRCodeData{
		MerchantID: originalMerchantID.String(),
		Type:       "subscription",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	parsedID, err := service.ParseSubscriptionQR(string(jsonData))
	require.NoError(t, err)
	assert.Equal(t, originalMerchantID, parsedID)
}
