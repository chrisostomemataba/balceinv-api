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