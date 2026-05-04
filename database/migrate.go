package database

import (
	"log"

	"github.com/chrisostomemataba/balceinv-api/models"
	"gorm.io/gorm"
)

// Migrate creates any tables that do not exist yet and adds any missing columns.
// It never drops columns or tables, so running this against your existing
// balce.db is completely safe — existing data is untouched.
func Migrate(db *gorm.DB) error {
	log.Println("Running migrations...")

	err := db.AutoMigrate(
		// Level 1 — no foreign keys, migrate first
		&models.Company{},      // new — must come before User and Settings
		&models.Role{},
		&models.Permission{},
		&models.Supplier{},

		// Level 2 — depends on Role and Company
		&models.User{},

		// Level 3 — depends on Role + Permission
		&models.RolePermission{},

		// Level 3 — depends on User + Permission
		&models.UserPermission{},

		// Level 3 — depends on User + Company
		&models.Session{},
		&models.LoginLog{},
		&models.Settings{},     // now has CompanyID

		// Catalog — no foreign keys, standalone reference data
		&models.CatalogProduct{}, // new

		// Level 3 — depends on Supplier + User
		&models.Purchase{},

		// Level 4 — depends on Purchase + Product
		&models.PurchaseItem{},

		// Level 2 — Product has no FK to anything above
		&models.Product{},

		// Level 3 — depends on Product
		&models.Barcode{},
		&models.PriceHistory{},
		&models.StockAlert{},

		// Level 3 — depends on User
		&models.Sale{},

		// Level 4 — depends on Sale + Product
		&models.SaleItem{},

		// Level 4 — depends on Product + User
		&models.StockMovement{},
	)

	if err != nil {
		return err
	}

	log.Println("Migrations complete.")
	return nil
}