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

	// Auth
	authService := services.NewAuthService(db, cfg.AccessTokenSecret, cfg.RefreshTokenSecret)
	authHandler := handlers.NewAuthHandler(authService)

	auth := app.Group("/api/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Get("/me", protected, authHandler.Me)

	// Products
	productRepo := repository.NewProductRepository(db)
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

	// Permissions
	permRepo := repository.NewPermissionRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	userRepo := repository.NewUserRepository(db)
	permService := services.NewPermissionService(permRepo, roleRepo, userRepo)
	permHandler := handlers.NewPermissionHandler(permService)

	perms := app.Group("/api/permissions", protected)
	perms.Get("/", permHandler.GetAll)
	perms.Get("/role/:id", permHandler.GetRolePermissions)
	perms.Get("/user/:id", permHandler.GetUserPermissions)
	perms.Post("/assign-role", permHandler.AssignToRole)
	perms.Post("/assign-user", permHandler.AssignToUser)
}