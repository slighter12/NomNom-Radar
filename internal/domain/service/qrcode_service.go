package service

import (
	"github.com/google/uuid"
)

// QRCodeService defines the interface for QR code generation and parsing services
type QRCodeService interface {
	// GenerateSubscriptionQR generates a QR code for merchant subscription
	GenerateSubscriptionQR(merchantID uuid.UUID) ([]byte, error)

	// ParseSubscriptionQR parses QR code data and returns the merchant ID
	ParseSubscriptionQR(qrData string) (uuid.UUID, error)
}
