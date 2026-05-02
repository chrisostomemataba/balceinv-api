package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chrisostomemataba/balceinv-api/models"
	"github.com/chrisostomemataba/balceinv-api/repository"
	"gorm.io/gorm"
)

type resolvedItem struct {
	input   SaleItemInput
	product models.Product
}

type SaleService struct {
	repo                *repository.SaleRepository
	productRepo         *repository.ProductRepository
	settingsRepo        *repository.SettingsRepository
	notificationService *NotificationService
}

// Update NewSaleService signature
func NewSaleService(
	repo *repository.SaleRepository,
	productRepo *repository.ProductRepository,
	settingsRepo *repository.SettingsRepository,
	notificationService *NotificationService,
) *SaleService {
	return &SaleService{
		repo:                repo,
		productRepo:         productRepo,
		settingsRepo:        settingsRepo,
		notificationService: notificationService,
	}
}

type SaleItemInput struct {
	ProductID   uint `json:"productId"`
	Quantity    int  `json:"quantity"`
	IsWholesale bool `json:"isWholesale"`
}

type CreateSaleInput struct {
	Items       []SaleItemInput `json:"items"`
	PaymentType string          `json:"paymentType"`
	SaleType    string          `json:"saleType"`
	AmountPaid  float64         `json:"amountPaid"`
	UseEFD      bool            `json:"useEFD"`
	UserID      uint
}

type SaleResult struct {
	ID            uint        `json:"id"`
	ReceiptNumber string      `json:"receipt_number"`
	Total         float64     `json:"total"`
	TaxAmount     float64     `json:"tax_amount"`
	PaymentType   string      `json:"payment_type"`
	AmountPaid    float64     `json:"amount_paid"`
	Change        float64     `json:"change"`
	ReceiptData   interface{} `json:"receipt_data"`
}

func (s *SaleService) GetAll(filters repository.SaleFilters) ([]models.Sale, error) {
	return s.repo.FindAll(filters)
}

func (s *SaleService) GetByID(id uint) (*models.Sale, error) {
	sale, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("sale not found")
	}
	return sale, err
}

func (s *SaleService) GetByDateRange(start, end time.Time) ([]models.Sale, error) {
	return s.repo.FindByDateRange(start, end)
}

func (s *SaleService) GetDailySummary(date time.Time) (map[string]interface{}, error) {
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999, date.Location())

	sales, err := s.repo.FindByDateRange(startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}

	totalRevenue := 0.0
	totalTax := 0.0
	for _, sale := range sales {
		totalRevenue += sale.TotalAmount
		totalTax += sale.TaxAmount
	}

	return map[string]interface{}{
		"sales":              sales,
		"total_revenue":      totalRevenue,
		"total_transactions": len(sales),
		"total_tax":          totalTax,
	}, nil
}

func (s *SaleService) GetMonthlySummary(year, month int) (map[string]interface{}, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year, time.Month(month+1), 0, 23, 59, 59, 999, time.UTC)

	sales, err := s.repo.FindByDateRange(start, end)
	if err != nil {
		return nil, err
	}

	totalRevenue := 0.0
	totalTax := 0.0
	for _, sale := range sales {
		totalRevenue += sale.TotalAmount
		totalTax += sale.TaxAmount
	}

	avg := 0.0
	if len(sales) > 0 {
		avg = totalRevenue / float64(len(sales))
	}

	return map[string]interface{}{
		"sales":               sales,
		"total_revenue":       totalRevenue,
		"total_transactions":  len(sales),
		"total_tax":           totalTax,
		"average_transaction": avg,
	}, nil
}

// CreateSale is the heart of the POS system. It fetches all products, validates
// stock, calculates totals, generates the receipt number, and commits everything
// in one transaction via the repository layer.
func (s *SaleService) CreateSale(input CreateSaleInput) (*SaleResult, error) {
	settings, _ := s.settingsRepo.GetSettings()

	// Resolve each product and check stock before touching the database
	resolved := make([]resolvedItem, 0, len(input.Items))
	for _, item := range input.Items {
		product, err := s.productRepo.FindByID(item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("product with ID %d not found", item.ProductID)
		}
		if product.Quantity < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for %s", product.Name)
		}
		resolved = append(resolved, resolvedItem{input: item, product: *product})
	}

	// Calculate the total — tax is already included in the price (inclusive VAT)
	taxRate := 18.0
	if settings != nil {
		taxRate = settings.TaxRate
	}

	total := 0.0
	for _, r := range resolved {
		price := r.product.Price
		if r.input.IsWholesale && r.product.WholesalePrice != nil {
			price = *r.product.WholesalePrice
		}
		total += price * float64(r.input.Quantity)
	}
	taxAmount := total * (taxRate / (100 + taxRate))

	// Validate cash payment covers the total
	change := 0.0
	if input.PaymentType == "cash" && input.AmountPaid > 0 {
		change = input.AmountPaid - total
		if change < 0 {
			return nil, fmt.Errorf("insufficient payment. required: %.2f, paid: %.2f", total, input.AmountPaid)
		}
	}

	receiptNumber, err := s.generateReceiptNumber(settings)
	if err != nil {
		return nil, err
	}

	saleType := input.SaleType
	if saleType == "" {
		saleType = "retail"
	}

	sale := &models.Sale{
		UserID:        input.UserID,
		ReceiptNumber: receiptNumber,
		TotalAmount:   total,
		TaxAmount:     taxAmount,
		PaymentType:   input.PaymentType,
		SaleType:      saleType,
	}

	// Build the sale items, stock deductions, and movement records before the transaction
	saleItems := make([]models.SaleItem, 0, len(resolved))
	movements := make([]models.StockMovement, 0, len(resolved))
	stockUpdates := map[uint]int{}

	ref := receiptNumber
	for _, r := range resolved {
		price := r.product.Price
		if r.input.IsWholesale && r.product.WholesalePrice != nil {
			price = *r.product.WholesalePrice
		}
		lineTotal := price * float64(r.input.Quantity)
		newQty := r.product.Quantity - r.input.Quantity

		saleItems = append(saleItems, models.SaleItem{
			ProductID:   r.product.ID,
			Quantity:    r.input.Quantity,
			UnitPrice:   price,
			TotalPrice:  lineTotal,
			IsWholesale: r.input.IsWholesale,
		})

		stockUpdates[r.product.ID] = newQty

		movements = append(movements, models.StockMovement{
			ProductID:   r.product.ID,
			Change:      -r.input.Quantity,
			NewQuantity: newQty,
			Reason:      "sale",
			Reference:   &ref,
			UserID:      &input.UserID,
		})
	}

	if err := s.repo.CreateWithItems(sale, saleItems, movements, stockUpdates); err != nil {
		return nil, err
	}

	// After s.repo.CreateWithItems succeeds, check stock levels for affected products.
	// This is what triggers notifications to appear in the frontend without any manual action.
	soldProductIDs := make([]uint, 0, len(resolved))
	for _, r := range resolved {
		soldProductIDs = append(soldProductIDs, r.product.ID)
	}
	// We deliberately ignore the error here — a failed stock check should never
	// cause a completed sale to return an error to the cashier.
	_ = s.notificationService.CheckStockLevels(soldProductIDs)

	return &SaleResult{
		ID:            sale.ID,
		ReceiptNumber: sale.ReceiptNumber,
		Total:         sale.TotalAmount,
		TaxAmount:     sale.TaxAmount,
		PaymentType:   sale.PaymentType,
		AmountPaid:    input.AmountPaid,
		Change:        change,
		ReceiptData:   s.buildReceiptData(sale, resolved, settings, change, taxRate),
	}, nil
}

func (s *SaleService) generateReceiptNumber(settings *models.Settings) (string, error) {
	format := "SALE-{TIMESTAMP}-{COUNTER}"
	if settings != nil && settings.ReceiptNumberFormat != "" {
		format = settings.ReceiptNumberFormat
	}

	count, err := s.repo.CountToday()
	if err != nil {
		return "", err
	}

	counter := count + 1
	timestamp := time.Now().UnixMilli()
	date := strings.ReplaceAll(time.Now().Format("2006-01-02"), "-", "")

	receipt := format
	receipt = strings.ReplaceAll(receipt, "{TIMESTAMP}", fmt.Sprintf("%d", timestamp))
	receipt = strings.ReplaceAll(receipt, "{COUNTER}", fmt.Sprintf("%04d", counter))
	receipt = strings.ReplaceAll(receipt, "{DATE}", date)

	return receipt, nil
}

func (s *SaleService) buildReceiptData(
	sale *models.Sale,
	resolved []resolvedItem,
	settings *models.Settings,
	change float64,
	taxRate float64,
) map[string]interface{} {
	businessName := "POS System"
	currency := "TZS"
	footer := "Thank you for your business!"

	if settings != nil {
		currency = settings.Currency
		if settings.Company.Name != "" {
			businessName = settings.Company.Name
		}
		if settings.Company.ReceiptFooter != nil {
			footer = *settings.Company.ReceiptFooter
		}
	}

	items := make([]map[string]interface{}, 0, len(resolved))
	for _, r := range resolved {
		price := r.product.Price
		if r.input.IsWholesale && r.product.WholesalePrice != nil {
			price = *r.product.WholesalePrice
		}
		items = append(items, map[string]interface{}{
			"name":         r.product.Name,
			"sku":          r.product.SKU,
			"quantity":     r.input.Quantity,
			"unit_price":   price,
			"total":        price * float64(r.input.Quantity),
			"is_wholesale": r.input.IsWholesale,
		})
	}

	return map[string]interface{}{
		"business_name":  businessName,
		"receipt_number": sale.ReceiptNumber,
		"date":           time.Now(),
		"payment_type":   sale.PaymentType,
		"items":          items,
		"subtotal":       sale.TotalAmount - sale.TaxAmount,
		"tax_amount":     sale.TaxAmount,
		"tax_rate":       taxRate,
		"total":          sale.TotalAmount,
		"change":         change,
		"currency":       currency,
		"receipt_footer": footer,
	}
}
