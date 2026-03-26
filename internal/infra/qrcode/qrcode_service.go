package qrcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"

	"radar/config"
	"radar/internal/domain/service"

	"github.com/google/uuid"
	goqrcode "github.com/yeqown/go-qrcode/v2"
)

type qrcodeService struct {
	size                 int
	errorCorrectionLevel string
}

const quietZoneModules = 4

// QRCodeData represents the QR code data structure
type QRCodeData struct {
	MerchantID string `json:"merchant_id"`
	Type       string `json:"type"`
}

// NewQRCodeService creates a new QR code service instance
func NewQRCodeService(cfg *config.Config) service.QRCodeService {
	if cfg.QRCode == nil {
		return &qrcodeService{
			size:                 256,
			errorCorrectionLevel: "M",
		}
	}

	level := "M"
	switch cfg.QRCode.ErrorCorrectionLevel {
	case "L", "M", "Q", "H":
		level = cfg.QRCode.ErrorCorrectionLevel
	}

	return &qrcodeService{
		size:                 cfg.QRCode.Size,
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
		return nil, fmt.Errorf("marshal QR payload: %w", err)
	}

	qrCode, err := goqrcode.NewWith(
		string(jsonData),
		mapErrorCorrectionLevel(s.errorCorrectionLevel),
	)
	if err != nil {
		return nil, fmt.Errorf("create QR code: %w", err)
	}

	writer := &pngWriter{requestedSize: s.size}
	if err := qrCode.Save(writer); err != nil {
		return nil, fmt.Errorf("render QR PNG: %w", err)
	}

	return writer.bytes, nil
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

func mapErrorCorrectionLevel(level string) goqrcode.EncodeOption {
	switch level {
	case "L":
		return goqrcode.WithErrorCorrectionLevel(goqrcode.ErrorCorrectionLow)
	case "Q":
		return goqrcode.WithErrorCorrectionLevel(goqrcode.ErrorCorrectionQuart)
	case "H":
		return goqrcode.WithErrorCorrectionLevel(goqrcode.ErrorCorrectionHighest)
	default:
		return goqrcode.WithErrorCorrectionLevel(goqrcode.ErrorCorrectionMedium)
	}
}

type pngWriter struct {
	requestedSize int
	bytes         []byte
}

func (w *pngWriter) Write(mat goqrcode.Matrix) error {
	bitmap := mat.Bitmap()
	realSize := mat.Width() + quietZoneModules*2
	outputSize := normalizeOutputSize(w.requestedSize, realSize)

	rect := image.Rectangle{
		Min: image.Point{},
		Max: image.Point{X: outputSize, Y: outputSize},
	}
	palette := color.Palette([]color.Color{color.White, color.Black})
	img := image.NewPaletted(rect, palette)
	fgColor := uint8(img.Palette.Index(color.Black))

	modulesPerPixel := float64(realSize) / float64(outputSize)
	for y := 0; y < outputSize; y++ {
		srcY := int(float64(y) * modulesPerPixel)
		for x := 0; x < outputSize; x++ {
			srcX := int(float64(x) * modulesPerPixel)
			if moduleAt(bitmap, srcX, srcY) {
				img.Pix[img.PixOffset(x, y)] = fgColor
			}
		}
	}

	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&buf, img); err != nil {
		return fmt.Errorf("encode QR PNG: %w", err)
	}

	w.bytes = buf.Bytes()

	return nil
}

func (w *pngWriter) Close() error {
	return nil
}

func normalizeOutputSize(size, realSize int) int {
	if size < 0 {
		return -size * realSize
	}
	if size < realSize {
		return realSize
	}

	return size
}

func moduleAt(bitmap [][]bool, moduleX, moduleY int) bool {
	if moduleX < quietZoneModules || moduleY < quietZoneModules {
		return false
	}

	bitmapY := moduleY - quietZoneModules
	if bitmapY >= len(bitmap) {
		return false
	}

	bitmapX := moduleX - quietZoneModules
	if bitmapX >= len(bitmap[bitmapY]) {
		return false
	}

	return bitmap[bitmapY][bitmapX]
}
