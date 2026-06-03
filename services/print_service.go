package services

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"
	"time"

	"github.com/chrisostomemataba/balceinv-api/models"
	"github.com/chrisostomemataba/balceinv-api/repository"
)

// PrintService writes ESC/POS receipts to a configured printer port.
// It is intentionally stateless — every call reads the current settings
// so that port changes in the settings UI take effect immediately.
type PrintService struct {
	saleRepo     *repository.SaleRepository
	settingsRepo *repository.SettingsRepository
}

func NewPrintService(
	saleRepo *repository.SaleRepository,
	settingsRepo *repository.SettingsRepository,
) *PrintService {
	return &PrintService{saleRepo: saleRepo, settingsRepo: settingsRepo}
}

// PrintReceipt is the single entry point called after a sale completes.
// It fetches the sale, loads settings+company, formats ESC/POS bytes,
// writes them to the configured port, and optionally pulses the cash drawer.
func (s *PrintService) PrintReceipt(saleID uint, openDrawer bool) error {
	settings, err := s.settingsRepo.GetSettings()
	if err != nil {
		return fmt.Errorf("could not load settings: %w", err)
	}

	// Respect the user's opt-in — if the printer is not enabled, do nothing.
	if !settings.PrinterEnabled {
		return nil
	}

	if settings.PrinterPort == "" {
		return fmt.Errorf("printer port is not configured")
	}

	sale, err := s.saleRepo.FindByID(saleID)
	if err != nil {
		return fmt.Errorf("sale not found: %w", err)
	}

	cols := 48 // default 80mm paper
	if settings.PrinterPaperWidth == 58 {
		cols = 32
	}

	buf := s.buildReceipt(sale, settings, cols)

	// Pulse the cash drawer before the receipt cuts so the drawer is open
	// by the time the cashier tears the receipt — standard POS behaviour.
	// The drawer is wired to the printer DK port and triggered via ESC/POS.
	if openDrawer && settings.OpenCashDrawer {
		buf = append(buf, escDrawerKick()...)
	}

	return s.writeToPort(settings.PrinterPort, buf)
}

// buildReceipt assembles the full ESC/POS byte sequence for the receipt.
func (s *PrintService) buildReceipt(sale *models.Sale, settings *models.Settings, cols int) []byte {
	var b []byte
	company := settings.Company
	currency := settings.Currency

	// ── Printer init ────────────────────────────────────────────────────────
	b = append(b, escInit()...)

	// ── Logo ────────────────────────────────────────────────────────────────
	// Logo is stored as a data URI (data:image/png;base64,...).
	// We decode and convert to ESC/POS raster bitmap.
	if company.Logo != nil && *company.Logo != "" {
		if logoBytes, err := logoFromDataURI(*company.Logo, cols); err == nil {
			b = append(b, logoBytes...)
			b = append(b, lf()...)
		}
	}

	// ── Header ──────────────────────────────────────────────────────────────
	b = append(b, escCenter()...)
	b = append(b, escBold(true)...)
	b = append(b, escDoubleHeight(true)...)
	b = append(b, []byte(center(company.Name, cols))...)
	b = append(b, lf()...)
	b = append(b, escDoubleHeight(false)...)
	b = append(b, escBold(false)...)

	if company.Address != nil && *company.Address != "" {
		b = append(b, []byte(center(*company.Address, cols))...)
		b = append(b, lf()...)
	}
	if company.Phone != nil && *company.Phone != "" {
		b = append(b, []byte(center("Tel: "+*company.Phone, cols))...)
		b = append(b, lf()...)
	}
	if company.TIN != nil && *company.TIN != "" {
		b = append(b, []byte(center("TIN: "+*company.TIN, cols))...)
		b = append(b, lf()...)
	}

	if company.ReceiptHeader != nil && *company.ReceiptHeader != "" {
		b = append(b, lf()...)
		b = append(b, []byte(center(*company.ReceiptHeader, cols))...)
		b = append(b, lf()...)
	}

	// ── Divider ─────────────────────────────────────────────────────────────
	b = append(b, escLeft()...)
	b = append(b, []byte(divider(cols, '-'))...)
	b = append(b, lf()...)

	// ── Receipt meta ────────────────────────────────────────────────────────
	b = append(b, []byte(fmt.Sprintf("Receipt : %s", sale.ReceiptNumber))...)
	b = append(b, lf()...)
	b = append(b, []byte(fmt.Sprintf("Date    : %s", sale.CreatedAt.Format("02/01/2006 15:04")))...)
	b = append(b, lf()...)
	b = append(b, []byte(fmt.Sprintf("Cashier : %s", sale.User.Name))...)
	b = append(b, lf()...)
	b = append(b, []byte(fmt.Sprintf("Payment : %s", strings.ToUpper(sale.PaymentType)))...)
	b = append(b, lf()...)

	// ── Divider ─────────────────────────────────────────────────────────────
	b = append(b, []byte(divider(cols, '-'))...)
	b = append(b, lf()...)

	// ── Column headers ──────────────────────────────────────────────────────
	b = append(b, escBold(true)...)
	b = append(b, []byte(itemHeader(cols))...)
	b = append(b, lf()...)
	b = append(b, escBold(false)...)
	b = append(b, []byte(divider(cols, '-'))...)
	b = append(b, lf()...)

	// ── Line items ──────────────────────────────────────────────────────────
	for _, item := range sale.Items {
		name := item.Product.Name
		if len(name) > cols {
			name = name[:cols-1]
		}
		b = append(b, []byte(name)...)
		b = append(b, lf()...)

		qtyPrice := fmt.Sprintf("  %d x %s %s",
			item.Quantity,
			formatAmount(item.UnitPrice, currency),
			wholesaleTag(item.IsWholesale),
		)
		total := formatAmount(item.TotalPrice, currency)
		b = append(b, []byte(leftRight(qtyPrice, total, cols))...)
		b = append(b, lf()...)
	}

	// ── Divider ─────────────────────────────────────────────────────────────
	b = append(b, []byte(divider(cols, '-'))...)
	b = append(b, lf()...)

	// ── Totals ──────────────────────────────────────────────────────────────
	subtotal := sale.TotalAmount - sale.TaxAmount
	b = append(b, []byte(leftRight("Subtotal", formatAmount(subtotal, currency), cols))...)
	b = append(b, lf()...)

	if settings.ShowTaxOnReceipt {
		taxLabel := fmt.Sprintf("VAT (%.0f%%)", settings.TaxRate)
		b = append(b, []byte(leftRight(taxLabel, formatAmount(sale.TaxAmount, currency), cols))...)
		b = append(b, lf()...)
	}

	b = append(b, []byte(divider(cols, '='))...)
	b = append(b, lf()...)

	b = append(b, escBold(true)...)
	b = append(b, []byte(leftRight("TOTAL", formatAmount(sale.TotalAmount, currency), cols))...)
	b = append(b, lf()...)
	b = append(b, escBold(false)...)

	// ── Footer ──────────────────────────────────────────────────────────────
	b = append(b, lf()...)

	if company.ReceiptFooter != nil && *company.ReceiptFooter != "" {
		b = append(b, escCenter()...)
		b = append(b, []byte(center(*company.ReceiptFooter, cols))...)
		b = append(b, lf()...)
		b = append(b, escLeft()...)
	}

	b = append(b, escCenter()...)
	b = append(b, []byte(center(time.Now().Format("02/01/2006 15:04:05"), cols))...)
	b = append(b, lf()...)
	b = append(b, lf()...)
	b = append(b, lf()...)

	// ── Paper cut ───────────────────────────────────────────────────────────
	b = append(b, escCut()...)

	return b
}

// writeToPort opens the printer device file and writes the ESC/POS bytes.
// On Windows the port is "COM3", "COM4" etc.
// On Linux it is "/dev/usb/lp0" (USB) or "/dev/ttyUSB0" (serial).
func (s *PrintService) writeToPort(port string, data []byte) error {
	f, err := os.OpenFile(port, os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("cannot open printer port %s: %w", port, err)
	}
	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		return fmt.Errorf("write to printer failed: %w", err)
	}
	return nil
}

// ── ESC/POS command helpers ──────────────────────────────────────────────────

func escInit() []byte       { return []byte{0x1B, 0x40} }
func lf() []byte            { return []byte{0x0A} }
func escCenter() []byte     { return []byte{0x1B, 0x61, 0x01} }
func escLeft() []byte       { return []byte{0x1B, 0x61, 0x00} }
func escCut() []byte        { return []byte{0x1D, 0x56, 0x41, 0x00} }
func escDrawerKick() []byte { return []byte{0x1B, 0x70, 0x00, 0x19, 0xFA} }

func escBold(on bool) []byte {
	if on {
		return []byte{0x1B, 0x45, 0x01}
	}
	return []byte{0x1B, 0x45, 0x00}
}

func escDoubleHeight(on bool) []byte {
	if on {
		return []byte{0x1D, 0x21, 0x01}
	}
	return []byte{0x1D, 0x21, 0x00}
}

// ── Layout helpers ───────────────────────────────────────────────────────────

func divider(cols int, char rune) string {
	return strings.Repeat(string(char), cols)
}

func center(s string, cols int) string {
	if len(s) >= cols {
		return s
	}
	pad := (cols - len(s)) / 2
	return strings.Repeat(" ", pad) + s
}

func leftRight(left, right string, cols int) string {
	space := cols - len(left) - len(right)
	if space < 1 {
		space = 1
	}
	return left + strings.Repeat(" ", space) + right
}

func itemHeader(cols int) string {
	return leftRight("Item", "Amount", cols)
}

func formatAmount(amount float64, currency string) string {
	formatted := fmt.Sprintf("%.0f", amount)
	n := len(formatted)
	result := ""
	for i, ch := range formatted {
		if i > 0 && (n-i)%3 == 0 {
			result += ","
		}
		result += string(ch)
	}
	return currency + " " + result
}

func wholesaleTag(isWholesale bool) string {
	if isWholesale {
		return "[W]"
	}
	return ""
}

// ── Logo rendering ───────────────────────────────────────────────────────────

// logoFromDataURI decodes a base64 data URI image and converts it to
// ESC/POS raster bit-image commands (GS v 0). The image is scaled to
// fit within the receipt width and converted to 1-bit black/white.
func logoFromDataURI(dataURI string, cols int) ([]byte, error) {
	idx := strings.Index(dataURI, ",")
	if idx < 0 {
		return nil, fmt.Errorf("invalid data URI")
	}

	imgBytes, err := base64.StdEncoding.DecodeString(dataURI[idx+1:])
	if err != nil {
		return nil, err
	}

	img, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, err
	}

	// Target width in dots: cols × 8, capped at 576 (80mm @203dpi)
	targetWidth := cols * 8
	if targetWidth > 576 {
		targetWidth = 576
	}

	bounds := img.Bounds()
	origW := bounds.Max.X - bounds.Min.X
	origH := bounds.Max.Y - bounds.Min.Y

	scale := float64(targetWidth) / float64(origW)
	targetHeight := int(math.Round(float64(origH) * scale))

	widthBytes := (targetWidth + 7) / 8
	raster := make([]byte, widthBytes*targetHeight)

	for y := 0; y < targetHeight; y++ {
		for x := 0; x < targetWidth; x++ {
			srcX := int(float64(x) / scale)
			srcY := int(float64(y) / scale)
			if srcX >= origW {
				srcX = origW - 1
			}
			if srcY >= origH {
				srcY = origH - 1
			}

			r, g, bVal, _ := img.At(bounds.Min.X+srcX, bounds.Min.Y+srcY).RGBA()
			gray := color.Gray{Y: uint8((r*299 + g*587 + bVal*114) / 1000 / 256)}
			if gray.Y < 128 {
				byteIdx := y*widthBytes + x/8
				bitIdx := uint(7 - (x % 8))
				raster[byteIdx] |= 1 << bitIdx
			}
		}
	}

	// GS v 0: 1D 76 30 m xL xH yL yH [data]
	xL := byte(widthBytes & 0xFF)
	xH := byte((widthBytes >> 8) & 0xFF)
	yL := byte(targetHeight & 0xFF)
	yH := byte((targetHeight >> 8) & 0xFF)

	cmd := []byte{0x1D, 0x76, 0x30, 0x00, xL, xH, yL, yH}
	cmd = append(cmd, raster...)
	return cmd, nil
}
