package services

import (
	"errors"
	"fmt"
	"mime/multipart"

	"github.com/chrisostomemataba/balceinv-api/models"
	"github.com/chrisostomemataba/balceinv-api/repository"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

type CreateProductInput struct {
	Name           string   `json:"name"`
	SKU            string   `json:"sku"`
	Barcode        *string  `json:"barcode"`
	Price          float64  `json:"price"`
	CostPrice      float64  `json:"cost_price"`
	Quantity       int      `json:"quantity"`
	MinStock       int      `json:"min_stock"`
	WholesalePrice *float64 `json:"wholesale_price"`
	WholesaleMin   int      `json:"wholesale_min"`
	Category       *string  `json:"category"`
	Unit           string   `json:"unit"`
	PiecesPerUnit  int      `json:"pieces_per_unit"`
}

type UpdateProductInput struct {
	Name           string   `json:"name"`
	Price          float64  `json:"price"`
	CostPrice      float64  `json:"cost_price"`
	MinStock       int      `json:"min_stock"`
	WholesalePrice *float64 `json:"wholesale_price"`
	WholesaleMin   int      `json:"wholesale_min"`
	Category       *string  `json:"category"`
	Unit           string   `json:"unit"`
	PiecesPerUnit  int      `json:"pieces_per_unit"`
}

type UploadResult struct {
	Created int                  `json:"created"`
	Errors  []map[string]string  `json:"errors"`
}

func (s *ProductService) GetAll(search, category string) ([]models.Product, error) {
	return s.repo.FindAll(search, category)
}

func (s *ProductService) GetByID(id uint) (*models.Product, error) {
	product, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("product not found")
	}
	return product, err
}

func (s *ProductService) Create(input CreateProductInput) (*models.Product, error) {
	// Reject duplicate SKU before attempting insert
	existing, err := s.repo.FindBySKU(input.SKU)
	if err == nil && existing != nil {
		return nil, errors.New("product with this SKU already exists")
	}

	unit := input.Unit
	if unit == "" {
		unit = "pcs"
	}
	piecesPerUnit := input.PiecesPerUnit
	if piecesPerUnit == 0 {
		piecesPerUnit = 1
	}
	minStock := input.MinStock
	if minStock == 0 {
		minStock = 5
	}
	wholesaleMin := input.WholesaleMin
	if wholesaleMin == 0 {
		wholesaleMin = 10
	}

	product := &models.Product{
		Name:           input.Name,
		SKU:            input.SKU,
		Barcode:        input.Barcode,
		Price:          input.Price,
		CostPrice:      input.CostPrice,
		Quantity:       input.Quantity,
		MinStock:       minStock,
		WholesalePrice: input.WholesalePrice,
		WholesaleMin:   wholesaleMin,
		Category:       input.Category,
		Unit:           unit,
		PiecesPerUnit:  piecesPerUnit,
	}

	if err := s.repo.Create(product); err != nil {
		return nil, err
	}

	// If initial stock was provided, record it as an adjustment movement
	if input.Quantity > 0 {
		ref := "Initial stock"
		s.repo.CreateStockMovement(&models.StockMovement{
			ProductID:   product.ID,
			Change:      input.Quantity,
			NewQuantity: input.Quantity,
			Reason:      "adjust",
			Reference:   &ref,
		})
	}

	return product, nil
}

func (s *ProductService) Update(id uint, input UpdateProductInput, userID *uint) (*models.Product, error) {
	product, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, errors.New("product not found")
	}
	if err != nil {
		return nil, err
	}

	// Record price history if the price actually changed
	if input.Price != 0 && input.Price != product.Price {
		oldPrice := product.Price
		newPrice := input.Price
		s.repo.CreatePriceHistory(&models.PriceHistory{
			ProductID: id,
			OldPrice:  &oldPrice,
			NewPrice:  &newPrice,
			UserID:    userID,
		})
	}

	product.Name = input.Name
	product.Price = input.Price
	product.CostPrice = input.CostPrice
	product.MinStock = input.MinStock
	product.WholesalePrice = input.WholesalePrice
	product.WholesaleMin = input.WholesaleMin
	product.Category = input.Category
	product.Unit = input.Unit
	product.PiecesPerUnit = input.PiecesPerUnit

	if err := s.repo.Update(product); err != nil {
		return nil, err
	}

	return product, nil
}

func (s *ProductService) Delete(id uint) error {
	_, err := s.repo.FindByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("product not found")
	}
	return s.repo.Delete(id)
}

func (s *ProductService) GetLowStock() ([]models.Product, error) {
	return s.repo.FindLowStock()
}

// UploadExcel reads an uploaded .xlsx file and creates products row by row.
// Rows that fail (duplicate SKU, bad data) are collected as errors and returned
// alongside the count of successful inserts — the upload never stops on a single error.
func (s *ProductService) UploadExcel(fileHeader *multipart.FileHeader) (*UploadResult, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, errors.New("could not open uploaded file")
	}
	defer file.Close()

	xlsx, err := excelize.OpenReader(file)
	if err != nil {
		return nil, errors.New("could not parse Excel file")
	}

	// Read from the first sheet, starting at row 2 (row 1 is the header)
	rows, err := xlsx.GetRows(xlsx.GetSheetName(0))
	if err != nil {
		return nil, errors.New("could not read sheet")
	}

	result := &UploadResult{Errors: []map[string]string{}}

	// Map header names to column indices so order does not matter
	if len(rows) < 2 {
		return result, nil
	}

	headers := rows[0]
	colIndex := map[string]int{}
	for i, h := range headers {
		colIndex[h] = i
	}

	col := func(row []string, name string) string {
		i, ok := colIndex[name]
		if !ok || i >= len(row) {
			return ""
		}
		return row[i]
	}

	for rowNum, row := range rows[1:] {
		sku := col(row, "sku")
		if sku == "" {
			result.Errors = append(result.Errors, map[string]string{
				"row": fmt.Sprintf("%d", rowNum+2), "error": "missing sku",
			})
			continue
		}

		input := CreateProductInput{
			Name:      col(row, "name"),
			SKU:       sku,
			Price:     parseFloat(col(row, "price")),
			CostPrice: parseFloat(col(row, "costPrice")),
			Quantity:  parseInt(col(row, "quantity")),
			MinStock:  parseInt(col(row, "minStock")),
			Unit:      col(row, "unit"),
		}

		if b := col(row, "barcode"); b != "" {
			input.Barcode = &b
		}
		if c := col(row, "category"); c != "" {
			input.Category = &c
		}
		if wp := parseFloat(col(row, "wholesalePrice")); wp > 0 {
			input.WholesalePrice = &wp
		}
		if wm := parseInt(col(row, "wholesaleMin")); wm > 0 {
			input.WholesaleMin = wm
		}
		if ppu := parseInt(col(row, "piecesPerUnit")); ppu > 0 {
			input.PiecesPerUnit = ppu
		}

		if _, err := s.Create(input); err != nil {
			result.Errors = append(result.Errors, map[string]string{
				"sku": sku, "error": err.Error(),
			})
			continue
		}

		result.Created++
	}

	return result, nil
}

// GetTemplate returns the bytes of a pre-filled .xlsx template file
// that the user can download, fill in, and upload back.
func (s *ProductService) GetTemplate() ([]byte, error) {
	f := excelize.NewFile()
	sheet := "Products"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"name", "sku", "barcode", "price", "costPrice",
		"quantity", "minStock", "wholesalePrice", "wholesaleMin",
		"category", "unit", "piecesPerUnit",
	}

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// One sample row so the user can see the expected format
	sample := []interface{}{
		"Sample Product", "SKU001", "1234567890", 100, 70,
		50, 10, 85, 20, "Drinks", "pcs", 1,
	}
	for i, v := range sample {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheet, cell, v)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// parseFloat and parseInt are small helpers used inside UploadExcel.
// They return zero on any parse error rather than crashing the upload.
func parseFloat(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}