package qrcode

import (
	"encoding/json"
	"fmt"

	"radar/internal/domain/service"

	"github.com/google/uuid"
	"github.com/skip2/go-qrcode"
)

type qrcodeService struct {
	size                 int
	errorCorrectionLevel qrcode.RecoveryLevel
}

// QRCodeData represents the QR code data structure
type QRCodeData struct {
	MerchantID string `json:"merchant_id"`
	Type       string `json:"type"`
}

// NewQRCodeService creates a new QR code service instance
func NewQRCodeService(size int, errorCorrectionLevel string) service.QRCodeService {
	// Set error correction level
	var level qrcode.RecoveryLevel
	switch errorCorrectionLevel {
	case "L":
		level = qrcode.Low
	case "M":
		level = qrcode.Medium
	case "Q":
		level = qrcode.High
	case "H":
		level = qrcode.Highest
	default:
		level = qrcode.Medium
	}

	return &qrcodeService{
		size:                 size,
		errorCorrectionLevel: level,
	}
}

// GenerateSubscriptionQR generates a QR code for merchant subscription
func (s *qrcodeService) GenerateSubscriptionQR(merchantID uuid.UUID) ([]byte, error) {
	// Create QR code data
	data := QRCodeData{
		MerchantID: merchantID.String(),
		Type:       "subscription",
	}

	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal QR code data: %w", err)
	}

	// Generate QR code
	qrCode, err := qrcode.New(string(jsonData), s.errorCorrectionLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create QR code: %w", err)
	}

	// Generate PNG image
	pngBytes, err := qrCode.PNG(s.size)
	if err != nil {
		return nil, fmt.Errorf("failed to generate PNG: %w", err)
	}

	return pngBytes, nil
}

// ParseSubscriptionQR parses QR code data and returns the merchant ID
func (s *qrcodeService) ParseSubscriptionQR(qrData string) (uuid.UUID, error) {
	var data QRCodeData
	if err := json.Unmarshal([]byte(qrData), &data); err != nil {
		return uuid.Nil, fmt.Errorf("failed to unmarshal QR code data: %w", err)
	}

	// Validate type
	if data.Type != "subscription" {
		return uuid.Nil, fmt.Errorf("invalid QR code type: %s", data.Type)
	}

	// Parse UUID
	merchantID, err := uuid.Parse(data.MerchantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to parse merchant ID: %w", err)
	}

	return merchantID, nil
}
