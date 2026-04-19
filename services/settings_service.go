package services

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/chrisostomemataba/balceinv-api/models"
	"github.com/chrisostomemataba/balceinv-api/repository"
	"gorm.io/gorm"
)

type SettingsService struct {
	repo *repository.SettingsRepository
}

func NewSettingsService(repo *repository.SettingsRepository) *SettingsService {
	return &SettingsService{repo: repo}
}

// GetOrCreate returns the settings row, creating defaults if none exist yet.
// This is safe to call on every request — it only inserts on the very first call.
func (s *SettingsService) GetOrCreate() (*models.Settings, error) {
	settings, err := s.repo.GetSettings()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return s.repo.CreateDefaults()
	}
	return settings, err
}

type UpdateSettingsInput struct {
	BusinessName              *string  `json:"businessName"`
	BusinessAddress           *string  `json:"businessAddress"`
	BusinessPhone             *string  `json:"businessPhone"`
	BusinessTIN               *string  `json:"businessTIN"`
	ReceiptHeader             *string  `json:"receiptHeader"`
	ReceiptFooter             *string  `json:"receiptFooter"`
	PrimaryColor              *string  `json:"primaryColor"`
	TaxRate                   *float64 `json:"taxRate"`
	Currency                  *string  `json:"currency"`
	CurrencySymbol            *string  `json:"currencySymbol"`
	DateFormat                *string  `json:"dateFormat"`
	ReceiptNumberFormat       *string  `json:"receiptNumberFormat"`
	EFDEnabled                *bool    `json:"efdEnabled"`
	EFDEndpoint               *string  `json:"efdEndpoint"`
	EFDApiKey                 *string  `json:"efdApiKey"`
	LowStockThreshold         *int     `json:"lowStockThreshold"`
	EmailNotificationsEnabled *bool    `json:"emailNotificationsEnabled"`
	NotificationEmail         *string  `json:"notificationEmail"`
	AlertSoundEnabled         *bool    `json:"alertSoundEnabled"`
	AlertOnLowStock           *bool    `json:"alertOnLowStock"`
	AlertOnOutOfStock         *bool    `json:"alertOnOutOfStock"`
	AlertOnDeadStock          *bool    `json:"alertOnDeadStock"`
	DeadStockDays             *int     `json:"deadStockDays"`
	PrintReceiptAutomatically *bool    `json:"printReceiptAutomatically"`
	ShowTaxOnReceipt          *bool    `json:"showTaxOnReceipt"`
	ShowBarcodesOnReceipt     *bool    `json:"showBarcodesOnReceipt"`
}

// Update applies only the fields that were actually sent in the request.
// Using a map[string]interface{} with GORM's Updates means zero-value fields
// like false or 0 are still written correctly — GORM's struct-based update
// skips zero values, which would silently ignore legitimate changes like
// setting efdEnabled to false or taxRate to 0.
func (s *SettingsService) Update(input UpdateSettingsInput, userID uint) (*models.Settings, error) {
	settings, err := s.GetOrCreate()
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{"updated_by": userID, "updated_at": time.Now()}

	if input.BusinessName != nil {
		updates["business_name"] = *input.BusinessName
	}
	if input.BusinessAddress != nil {
		updates["business_address"] = *input.BusinessAddress
	}
	if input.BusinessPhone != nil {
		updates["business_phone"] = *input.BusinessPhone
	}
	if input.BusinessTIN != nil {
		updates["business_tin"] = *input.BusinessTIN
	}
	if input.ReceiptHeader != nil {
		updates["receipt_header"] = *input.ReceiptHeader
	}
	if input.ReceiptFooter != nil {
		updates["receipt_footer"] = *input.ReceiptFooter
	}
	if input.PrimaryColor != nil {
		updates["primary_color"] = *input.PrimaryColor
	}
	if input.TaxRate != nil {
		updates["tax_rate"] = *input.TaxRate
	}
	if input.Currency != nil {
		updates["currency"] = *input.Currency
	}
	if input.CurrencySymbol != nil {
		updates["currency_symbol"] = *input.CurrencySymbol
	}
	if input.DateFormat != nil {
		updates["date_format"] = *input.DateFormat
	}
	if input.ReceiptNumberFormat != nil {
		updates["receipt_number_format"] = *input.ReceiptNumberFormat
	}
	if input.EFDEnabled != nil {
		updates["efd_enabled"] = *input.EFDEnabled
	}
	if input.EFDEndpoint != nil {
		updates["efd_endpoint"] = *input.EFDEndpoint
	}
	if input.EFDApiKey != nil {
		updates["efd_api_key"] = *input.EFDApiKey
	}
	if input.LowStockThreshold != nil {
		updates["low_stock_threshold"] = *input.LowStockThreshold
	}
	if input.EmailNotificationsEnabled != nil {
		updates["email_notifications_enabled"] = *input.EmailNotificationsEnabled
	}
	if input.NotificationEmail != nil {
		updates["notification_email"] = *input.NotificationEmail
	}
	if input.AlertSoundEnabled != nil {
		updates["alert_sound_enabled"] = *input.AlertSoundEnabled
	}
	if input.AlertOnLowStock != nil {
		updates["alert_on_low_stock"] = *input.AlertOnLowStock
	}
	if input.AlertOnOutOfStock != nil {
		updates["alert_on_out_of_stock"] = *input.AlertOnOutOfStock
	}
	if input.AlertOnDeadStock != nil {
		updates["alert_on_dead_stock"] = *input.AlertOnDeadStock
	}
	if input.DeadStockDays != nil {
		updates["dead_stock_days"] = *input.DeadStockDays
	}
	if input.PrintReceiptAutomatically != nil {
		updates["print_receipt_automatically"] = *input.PrintReceiptAutomatically
	}
	if input.ShowTaxOnReceipt != nil {
		updates["show_tax_on_receipt"] = *input.ShowTaxOnReceipt
	}
	if input.ShowBarcodesOnReceipt != nil {
		updates["show_barcodes_on_receipt"] = *input.ShowBarcodesOnReceipt
	}

	return s.repo.Update(settings.ID, updates)
}

type EFDTestResult struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// TestEFD sends a test payload to the EFD endpoint and records the result.
// This is the TRA fiscal device integration test your TypeScript version implemented.
func (s *SettingsService) TestEFD(endpoint, apiKey string) (*EFDTestResult, error) {
	settings, err := s.GetOrCreate()
	if err != nil {
		return nil, err
	}

	payload := fmt.Sprintf(`{"test":true,"timestamp":"%s"}`, time.Now().Format(time.RFC3339))
	req, err := http.NewRequest("POST", endpoint, strings.NewReader(payload))
	if err != nil {
		s.recordEFDTest(settings.ID, "failed")
		return &EFDTestResult{Status: "failed", Message: err.Error()}, nil
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.recordEFDTest(settings.ID, "failed")
		return &EFDTestResult{Status: "failed", Message: err.Error()}, nil
	}
	defer resp.Body.Close()

	if !isSuccessStatus(resp.StatusCode) {
		s.recordEFDTest(settings.ID, "failed")
		return &EFDTestResult{
			Status:  "failed",
			Message: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status),
		}, nil
	}

	s.recordEFDTest(settings.ID, "success")
	return &EFDTestResult{Status: "success", Message: "Successfully connected to EFD endpoint"}, nil
}

func (s *SettingsService) recordEFDTest(settingsID uint, status string) {
	now := time.Now()
	s.repo.Update(settingsID, map[string]interface{}{
		"efd_last_test_date": now,
		"efd_test_status":    status,
	})
}

func isSuccessStatus(code int) bool {
	return code >= 200 && code < 300
}

// UploadLogo converts the uploaded image file to a base64 data URI and stores
// it directly in the settings row. This keeps the logo self-contained in the
// database without needing a separate file storage system — the same approach
// your TypeScript version used.
func (s *SettingsService) UploadLogo(fileHeader *multipart.FileHeader) (string, error) {
	settings, err := s.GetOrCreate()
	if err != nil {
		return "", err
	}

	file, err := fileHeader.Open()
	if err != nil {
		return "", errors.New("could not open uploaded file")
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return "", errors.New("could not read file")
	}

	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/png"
	}

	logoURL := fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data))

	if _, err := s.repo.Update(settings.ID, map[string]interface{}{"business_logo": logoURL}); err != nil {
		return "", err
	}

	return logoURL, nil
}