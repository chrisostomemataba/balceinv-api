package routes

import (
	"github.com/chrisostomemataba/balceinv-api/config"
	"github.com/chrisostomemataba/balceinv-api/handlers"
	"github.com/chrisostomemataba/balceinv-api/middleware"
	"github.com/chrisostomemataba/balceinv-api/repository"
	"github.com/chrisostomemataba/balceinv-api/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func Setup(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	protected := middleware.Protected(cfg.AccessTokenSecret)

	// --- Repositories ---
	// All repositories are created first because services depend on them.
	// Creating them here in one place means we never accidentally create
	// two instances of the same repository pointing at the same database.
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	permRepo := repository.NewPermissionRepository(db)
	productRepo := repository.NewProductRepository(db)
	saleRepo := repository.NewSaleRepository(db)
	stockRepo := repository.NewStockMovementRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	reportRepo := repository.NewReportRepository(db)

	// --- Auth ---
	authService := services.NewAuthService(db, cfg.AccessTokenSecret, cfg.RefreshTokenSecret)
	authHandler := handlers.NewAuthHandler(authService)

	auth := app.Group("/api/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Get("/me", protected, authHandler.Me)

	// --- Users ---
	userService := services.NewUserService(userRepo, roleRepo)
	userHandler := handlers.NewUserHandler(userService)

	users := app.Group("/api/users", protected)
	users.Get("/", userHandler.GetAll)
	users.Post("/update-password", userHandler.UpdatePassword)
	users.Get("/:id", userHandler.GetByID)
	users.Post("/", userHandler.Create)
	users.Put("/:id", userHandler.Update)
	users.Delete("/:id", userHandler.Delete)

	// --- Roles ---
	roleService := services.NewRoleService(roleRepo, userRepo)
	roleHandler := handlers.NewRoleHandler(roleService)

	roles := app.Group("/api/roles", protected)
	roles.Get("/", roleHandler.GetAll)
	roles.Post("/assign", roleHandler.AssignRole)
	roles.Get("/:id", roleHandler.GetByID)
	roles.Post("/", roleHandler.Create)
	roles.Put("/:id", roleHandler.Update)
	roles.Delete("/:id", roleHandler.Delete)

	// --- Permissions ---
	permService := services.NewPermissionService(permRepo, roleRepo, userRepo)
	permHandler := handlers.NewPermissionHandler(permService)

	perms := app.Group("/api/permissions", protected)
	perms.Get("/", permHandler.GetAll)
	perms.Get("/role/:id", permHandler.GetRolePermissions)
	perms.Get("/user/:id", permHandler.GetUserPermissions)
	perms.Post("/assign-role", permHandler.AssignToRole)
	perms.Post("/assign-user", permHandler.AssignToUser)

	// --- Products ---
	productService := services.NewProductService(productRepo)
	productHandler := handlers.NewProductHandler(productService)

	products := app.Group("/api/products", protected)
	products.Get("/", productHandler.GetAll)
	products.Get("/low-stock", productHandler.GetLowStock)
	products.Get("/template", productHandler.GetTemplate)
	products.Get("/:id", productHandler.GetByID)
	products.Post("/", productHandler.Create)
	products.Post("/upload", productHandler.UploadExcel)
	products.Put("/:id", productHandler.Update)
	products.Delete("/:id", productHandler.Delete)

	// --- Notifications ---
	// Notifications are declared before sales because the sale service
	// needs notificationService as a dependency. Declaring it first
	// means we can pass it directly without a forward reference issue.
	notificationService := services.NewNotificationService(notificationRepo, settingsRepo, productRepo)
	notificationHandler := handlers.NewNotificationHandler(notificationService)

	notifs := app.Group("/api/notifications", protected)
	notifs.Get("/", notificationHandler.GetAll)
	notifs.Get("/count", notificationHandler.GetCount)
	notifs.Post("/mark-all-seen", notificationHandler.MarkAllAsSeen)
	notifs.Delete("/clear-seen", notificationHandler.ClearSeen)
	notifs.Post("/:id/mark-seen", notificationHandler.MarkAsSeen)
	notifs.Delete("/:id", notificationHandler.Delete)

	// --- Sales ---
	// Sales is wired after notifications so notificationService is available.
	// settingsRepo is passed so the sale service can read tax rate and receipt format.
	// notificationService is passed so stock alerts fire automatically after each sale.
	saleService := services.NewSaleService(saleRepo, productRepo, settingsRepo, notificationService)
	saleHandler := handlers.NewSaleHandler(saleService)

	sales := app.Group("/api/sales", protected)
	sales.Get("/", saleHandler.GetAll)
	sales.Get("/daily", saleHandler.GetDaily)
	sales.Get("/monthly", saleHandler.GetMonthly)
	sales.Get("/date-range", saleHandler.GetByDateRange)
	sales.Get("/:id", saleHandler.GetByID)
	sales.Post("/", saleHandler.Create)

	// --- Stock Movements ---
	stockService := services.NewStockMovementService(stockRepo, productRepo)
	stockHandler := handlers.NewStockMovementHandler(stockService)

	stock := app.Group("/api/stock-movements", protected)
	stock.Get("/", stockHandler.GetAll)
	stock.Get("/summary", stockHandler.GetSummary)
	stock.Get("/date-range", stockHandler.GetByDateRange)
	stock.Get("/product/:id", stockHandler.GetByProduct)
	stock.Get("/:id", stockHandler.GetByID)
	stock.Post("/", stockHandler.Create)

	// --- Settings ---
	settingsService := services.NewSettingsService(settingsRepo)
	settingsHandler := handlers.NewSettingsHandler(settingsService)

	settingsGroup := app.Group("/api/settings", protected)
	settingsGroup.Get("/", settingsHandler.Get)
	settingsGroup.Put("/", settingsHandler.Update)
	settingsGroup.Post("/test-efd", settingsHandler.TestEFD)
	settingsGroup.Post("/upload-logo", settingsHandler.UploadLogo)

	//--- Reports ---
	reportService := services.NewReportService(reportRepo)
	reportHandler := handlers.NewReportHandler(reportService)

	reports := app.Group("/api/reports", protected)
	reports.Get("/sales-summary", reportHandler.GetSalesSummary)
	reports.Get("/top-products", reportHandler.GetTopProducts)
	reports.Get("/sales-by-user", reportHandler.GetSalesByUser)
	reports.Get("/inventory", reportHandler.GetInventory)
	reports.Get("/financial", reportHandler.GetFinancial)
	reports.Get("/daily-trend", reportHandler.GetDailyTrend)

	//--- Dashboard ---
	dashboardHandler := handlers.NewDashboardHandler(db)
	app.Get("/api/dashboard", protected, dashboardHandler.Get)

	// --- Setup ---
	setup := handlers.NewSetupHandler(services.NewSetupService(db))

	app.Get("/api/setup/status", setup.Status)
	app.Post("/api/setup", setup.Run)
}
