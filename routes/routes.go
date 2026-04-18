package routes

import (
	"github.com/chrisostomemataba/balceinv-api/config"
	"github.com/chrisostomemataba/balceinv-api/handlers"
	"github.com/chrisostomemataba/balceinv-api/middleware"
	"github.com/chrisostomemataba/balceinv-api/services"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func Setup(app *fiber.App, db *gorm.DB, cfg *config.Config) {
	authService := services.NewAuthService(db, cfg.AccessTokenSecret, cfg.RefreshTokenSecret)
	authHandler := handlers.NewAuthHandler(authService)

	auth := app.Group("/api/auth")
	auth.Post("/login", authHandler.Login)
	auth.Post("/logout", authHandler.Logout)
	auth.Post("/refresh", authHandler.Refresh)
	auth.Get("/me", middleware.Protected(cfg.AccessTokenSecret), authHandler.Me)
}