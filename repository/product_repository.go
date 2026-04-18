package repository

import (
	"github.com/chrisostomemataba/balceinv-api/models"
	"gorm.io/gorm"
)

type ProductRepository struct {
	db *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

// FindAll returns products filtered by optional search and category query params.
// The LIKE search checks both name and sku — same behaviour as your TypeScript getAll.
func (r *ProductRepository) FindAll(search, category string) ([]models.Product, error) {
	query := r.db.Order("created_at DESC")

	if search != "" {
		query = query.Where("name LIKE ? OR sku LIKE ?", "%"+search+"%", "%"+search+"%")
	}

	if category != "" {
		query = query.Where("category = ?", category)
	}

	var products []models.Product
	err := query.Find(&products).Error
	return products, err
}

// FindByID returns one product with its barcodes and last 10 stock movements preloaded.
func (r *ProductRepository) FindByID(id uint) (*models.Product, error) {
	var product models.Product
	err := r.db.
		Preload("Barcodes").
		Preload("StockMovements", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC").Limit(10)
		}).
		First(&product, id).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// FindBySKU checks if a product with this SKU already exists — used before creating.
func (r *ProductRepository) FindBySKU(sku string) (*models.Product, error) {
	var product models.Product
	err := r.db.Where("sku = ?", sku).First(&product).Error
	if err != nil {
		return nil, err
	}
	return &product, nil
}

// Create inserts a new product row and returns the created record with its new ID.
func (r *ProductRepository) Create(product *models.Product) error {
	return r.db.Create(product).Error
}

// Update saves changes to an existing product row.
func (r *ProductRepository) Update(product *models.Product) error {
	return r.db.Save(product).Error
}

// Delete removes a product by ID.
func (r *ProductRepository) Delete(id uint) error {
	return r.db.Delete(&models.Product{}, id).Error
}

// FindLowStock returns all products where quantity is at or below their minStock threshold.
func (r *ProductRepository) FindLowStock() ([]models.Product, error) {
	var products []models.Product
	err := r.db.Where("quantity <= min_stock").Order("quantity ASC").Find(&products).Error
	return products, err
}

// CreateStockMovement inserts a stock movement record.
// Called every time stock changes — whether from a sale, adjustment, or upload.
func (r *ProductRepository) CreateStockMovement(movement *models.StockMovement) error {
	return r.db.Create(movement).Error
}

// CreatePriceHistory inserts a price history record when a product price changes.
func (r *ProductRepository) CreatePriceHistory(history *models.PriceHistory) error {
	return r.db.Create(history).Error
}