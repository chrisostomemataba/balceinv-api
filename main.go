package main

import (
	"log"

	"github.com/chrisostomemataba/balceinv-api/config"
	"github.com/chrisostomemataba/balceinv-api/database"
	"github.com/chrisostomemataba/balceinv-api/routes"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// Step 1 — Load configuration from .env
	cfg := config.Load()

	// Step 2 — Connect to the SQLite database
	db, err := database.Connect(cfg.DBPath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Step 3 — Run migrations so all tables exist before the server accepts requests
	if err := database.Migrate(db); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Step 4 — Create the Fiber app
	// JSONEncoder and JSONDecoder use the standard library — no extra dependency needed.
	app := fiber.New(fiber.Config{
		AppName: "BalceInv API",
	})

	// Step 5 — Register global middleware
	// recover catches any panic in a handler and returns a 500 instead of crashing the server.
	// logger prints each request method, path, status, and duration to the terminal.
	app.Use(recover.New())
	app.Use(logger.New())

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:3000",
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true, // required because you are sending cookies
		ExposeHeaders:    "Set-Cookie",
	}))

	// Step 6 — Health check route so you can confirm the server is running
	// Hit GET http://localhost:8080/health in your browser or with curl.
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"version": "1.0.0",
		})
	})

	// Step 7 — Setup routes
	routes.Setup(app, db, cfg)

	// Step 8 — Start listening
	// The colon before the port number is required by Go's net package — it means
	// "listen on all network interfaces on this port".
	log.Printf("Server starting on port %s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
