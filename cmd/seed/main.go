package main

import (
	"log"
	"os"
	

	"github.com/chrisostomemataba/balceinv-api/database"
	"github.com/chrisostomemataba/balceinv-api/models"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

)

func main() {
	// Load .env file so DB_PATH and secrets are available
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file, using system environment variables")
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./balce.db"
	}

	// Connect then migrate — tables must exist before we insert seed data
	db, err := database.Connect(dbPath)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Starting seed...")

	seedRoles(db)
	seedPermissions(db)
	seedSuperAdmin(db)
	seedSettings(db)

	log.Println("Seed complete.")
}

// seedRoles creates the three base roles.
// FirstOrCreate means: if a role with this Name already exists, do nothing.
// Running the seed twice will never create duplicates.
func seedRoles(db *gorm.DB) {
	roles := []string{"SuperAdmin", "Manager", "Cashier"}

	for _, name := range roles {
		role := models.Role{Name: name}
		result := db.Where("name = ?", name).FirstOrCreate(&role)
		if result.Error != nil {
			log.Printf("Error seeding role %s: %v", name, result.Error)
			continue
		}
		if result.RowsAffected > 0 {
			log.Printf("Created role: %s", name)
		} else {
			log.Printf("Role already exists, skipping: %s", name)
		}
	}
}

// seedPermissions builds every resource+action combination and inserts them,
// then assigns all permissions to the SuperAdmin role.
// This mirrors exactly what your TypeScript seed script did.
func seedPermissions(db *gorm.DB) {
	resources := []string{
		"products", "sales", "users", "roles",
		"stock_movements", "reports", "settings", "notifications",
	}
	actions := []string{"view", "create", "edit", "delete"}

	for _, resource := range resources {
		for _, action := range actions {
			name := resource + ":" + action // e.g. "products:create"
			perm := models.Permission{
				Name:     name,
				Resource: resource,
				Action:   action,
			}
			db.Where("name = ?", name).FirstOrCreate(&perm)
		}
	}

	log.Println("Permissions seeded.")

	// Find SuperAdmin role
	var superAdmin models.Role
	if err := db.Where("name = ?", "SuperAdmin").First(&superAdmin).Error; err != nil {
		log.Println("SuperAdmin role not found, skipping permission assignment")
		return
	}

	// Assign every permission to SuperAdmin
	var allPerms []models.Permission
	db.Find(&allPerms)

	for _, perm := range allPerms {
		rp := models.RolePermission{
			RoleID:       superAdmin.ID,
			PermissionID: perm.ID,
		}
		// Only insert if this exact combination does not already exist
		db.Where("role_id = ? AND permission_id = ?", rp.RoleID, rp.PermissionID).
			FirstOrCreate(&rp)
	}

	log.Println("SuperAdmin permissions assigned.")
}

// seedSuperAdmin creates the default admin account.
// Email: admin@balceinv.com  Password: admin123
// Change the password immediately after first login.
func seedSuperAdmin(db *gorm.DB) {
	var superAdmin models.Role
	if err := db.Where("name = ?", "SuperAdmin").First(&superAdmin).Error; err != nil {
		log.Println("SuperAdmin role missing, cannot create admin user")
		return
	}

	// Check if admin already exists — if so, skip entirely
	var existing models.User
	if db.Where("email = ?", "admin@balceinv.com").First(&existing).Error == nil {
		log.Println("Super admin already exists, skipping.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	admin := models.User{
		Name:         "Super Admin",
		Email:        "admin@balceinv.com",
		PasswordHash: string(hash),
		RoleID:       superAdmin.ID,
	}

	if err := db.Create(&admin).Error; err != nil {
		log.Printf("Error creating super admin: %v", err)
	} else {
		log.Println("Super admin created — email: admin@balceinv.com  password: admin123")
	}
}

// seedSettings inserts one default settings row if the table is empty.
// There is always exactly one settings row — the shop owner edits it from the UI.
func seedSettings(db *gorm.DB) {
	var existing models.Settings
	if db.First(&existing).Error == nil {
		log.Println("Settings already exist, skipping.")
		return
	}

	footer := "Thank you for your business!"

	settings := models.Settings{
		TaxRate:             18.0,
		Currency:            "TZS",
		CurrencySymbol:      "TZS",
		DateFormat:          "DD/MM/YYYY",
		ReceiptNumberFormat: "SALE-{TIMESTAMP}-{COUNTER}",
		ReceiptFooter:       &footer,
		LowStockThreshold:   5,
		AlertSoundEnabled:   true,
		AlertOnLowStock:     true,
		AlertOnOutOfStock:   true,
		ShowTaxOnReceipt:    true,
	}

	if err := db.Create(&settings).Error; err != nil {
		log.Printf("Error creating settings: %v", err)
	} else {
		log.Println("Default settings created.")
	}
}