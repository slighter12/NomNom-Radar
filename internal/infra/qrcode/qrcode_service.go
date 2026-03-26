package qrcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"radar/config"
	"radar/internal/domain/service"

	"github.com/google/uuid"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

type qrcodeService struct {
	errorCorrectionOption qrcode.EncodeOption
}

const (
	qrBlockWidth     = 20
	quietZoneModules = 4
)

// QRCodeData represents the QR code data structure
type QRCodeData struct {
	MerchantID string `json:"merchant_id"`
	Type       string `json:"type"`
}

// NewQRCodeService creates a new QR code service instance
func NewQRCodeService(cfg *config.Config) service.QRCodeService {
	if cfg.QRCode == nil {
		return &qrcodeService{
			errorCorrectionOption: qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium),
		}
	}

	option := qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium)
	switch cfg.QRCode.ErrorCorrectionLevel {
	case "L":
		option = qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionLow)
	case "Q":
		option = qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionQuart)
	case "H":
		option = qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionHighest)
	}

	return &qrcodeService{
		errorCorrectionOption: option,
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
		return nil, fmt.Errorf("marshal QR payload: %w", err)
	}

	qrCode, err := qrcode.NewWith(
		string(jsonData),
		s.errorCorrectionOption,
	)
	if err != nil {
		return nil, fmt.Errorf("create QR code: %w", err)
	}

	var buf bytes.Buffer
	writer := standard.NewWithWriter(
		bufferWriteCloser{Writer: &buf},
		standard.WithQRWidth(qrBlockWidth),
		standard.WithBorderWidth(qrBlockWidth*quietZoneModules),
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
	)
	if err := qrCode.Save(writer); err != nil {
		return nil, fmt.Errorf("render QR PNG: %w", err)
	}

	return buf.Bytes(), nil
}

// ParseSubscriptionQR parses QR code data and returns the merchant ID
func (s *qrcodeService) ParseSubscriptionQR(qrData string) (uuid.UUID, error) {
	var data QRCodeData
	if err := json.Unmarshal([]byte(qrData), &data); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal QR payload: %w", err)
	}

	// Validate type
	if data.Type != "subscription" {
		return uuid.Nil, fmt.Errorf("invalid QR code type: %s", data.Type)
	}

	// Parse UUID
	merchantID, err := uuid.Parse(data.MerchantID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("parse merchant ID from QR payload: %w", err)
	}

	return merchantID, nil
}

type bufferWriteCloser struct {
	io.Writer
}

func (w bufferWriteCloser) Close() error {
	// bytes.Buffer does not hold external resources, so Close is a no-op adapter.
	return nil
}
