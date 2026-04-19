package repository

import (
	"github.com/chrisostomemataba/balceinv-api/models"
	"gorm.io/gorm"
)

type SettingsRepository struct {
	db *gorm.DB
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

func (r *SettingsRepository) GetSettings() (*models.Settings, error) {
	var settings models.Settings
	err := r.db.First(&settings).Error
	if err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *SettingsRepository) CreateDefaults() (*models.Settings, error) {
	footer := "Thank you for your business!"
	format := "SALE-{TIMESTAMP}-{COUNTER}"
	color := "#3b82f6"

	settings := &models.Settings{
		PrimaryColor:        &color,
		TaxRate:             18.0,
		Currency:            "TZS",
		CurrencySymbol:      "TZS",
		DateFormat:          "DD/MM/YYYY",
		ReceiptNumberFormat: format,
		ReceiptFooter:       &footer,
		EFDEnabled:          false,
		LowStockThreshold:   5,
		AlertSoundEnabled:   true,
		AlertOnLowStock:     true,
		AlertOnOutOfStock:   true,
		ShowTaxOnReceipt:    true,
	}

	if err := r.db.Create(settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (r *SettingsRepository) Update(id uint, updates map[string]interface{}) (*models.Settings, error) {
	if err := r.db.Model(&models.Settings{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	var settings models.Settings
	if err := r.db.First(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}
