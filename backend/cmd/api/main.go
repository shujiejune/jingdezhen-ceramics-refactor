package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jingdezhen-ceramics-backend/internal/api"
	"jingdezhen-ceramics-backend/internal/config"
	"jingdezhen-ceramics-backend/internal/modules/ceramicstory"
	"jingdezhen-ceramics-backend/internal/modules/engage"
	"jingdezhen-ceramics-backend/internal/modules/gallery"
	"jingdezhen-ceramics-backend/internal/modules/notification"
	"jingdezhen-ceramics-backend/internal/modules/user"
	"jingdezhen-ceramics-backend/internal/ws"
	"jingdezhen-ceramics-backend/pkg/email"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		log.Fatalf("Could not load config: %v", err)
	}

	// --- Create a cancellable context for the entire application lifecycle ---
	// This context will be passed to long-running background services.
	ctx, cancel := context.WithCancel(context.Background())
	// We will call cancel() later when we receive a shutdown signal.
	defer cancel()

	// --- Initialize Fiber App ---
	app := fiber.New()

	// --- Middleware ---
	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.ClientOrigin + ", http://localhost:5173",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowMethods:     "GET,POST,PUT,DELETE,PATCH,OPTIONS",
		AllowCredentials: true,
	}))

	// --- Database connection ---
	dbConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to parse database configuration: %v\n", err)
	}

	dbPool, err := pgxpool.NewWithConfig(context.Background(), dbConfig)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbPool.Close() // Ensure pool is closed when main exits

	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}
	log.Println("Successfully connected to the database!")

	// --- WebSocket Initialization ---
	// 1. Create the WebSocket Hub (our service).
	wsHub := ws.NewHub()
	// 2. Start the Hub's main event loop in a background goroutine.
	go wsHub.Run(ctx)
	// 3. Create the WebSocket handler, passing it the Hub.
	wsHandler := ws.NewHandler(wsHub)

	// --- Dependency injection ---
	// 1. Initialize Google OAuth Config
	googleOAuthConfig := &oauth2.Config{
		RedirectURL:  cfg.GoogleOAuthRedirectURL,
		ClientID:     cfg.GoogleOAuthClientID,
		ClientSecret: cfg.GoogleOAuthClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	// 2. Initialize other services
	sesSender, err := email.NewSESV2Sender(context.Background(), cfg.AWSRegion, cfg.AdminEmail)
	if err != nil {
		log.Fatalf("Failed to create SES sender: %v", err)
	}
	templateManager, err := email.NewTemplateManager()
	if err != nil {
		log.Fatalf("Failed to parse email templates: %v", err)
	}

	userRepo := user.NewRepository(dbPool)
	userService := user.NewService(
		userRepo,
		sesSender,
		templateManager,
		cfg.JWTSecret,
		cfg.ClientOrigin,
		cfg.AdminEmail,
		googleOAuthConfig,
	)
	userHandler := user.NewHandler(userService)
	// You'll also need an admin handler if it's separate
	// adminHandler := user.NewAdminHandler(userService, other admin services)

	notifRepo := notification.NewRepository(dbPool)
	notifService := notification.NewService(notifRepo, userRepo, wsHub)
	notifHandler := notification.NewHandler(notifService)

	ceramicStoryRepo := ceramicstory.NewRepository(dbPool)
	ceramicStoryService := ceramicstory.NewService(ceramicStoryRepo)
	ceramicStoryHandler := ceramicstory.NewHandler(ceramicStoryService)

	galleryRepo := gallery.NewRepository(dbPool)
	galleryService := gallery.NewService(galleryRepo)
	galleryHandler := gallery.NewHandler(galleryService)

	engageRepo := engage.NewRepository(dbPool)
	engageService := engage.NewService(engageRepo)
	engageHandler := engage.NewHandler(engageService)

	// --- Initialize router, passing all handlers and other necessary dependencies ---
	api.SetupRoutes(app, cfg.JWTSecret,
		wsHandler,
		userHandler,
		notifHandler,
		ceramicStoryHandler,
		galleryHandler,
		engageHandler,
	)

	// --- Start server (graceful shutdown logic) ---
	// Listen on a channel for interrupt signals.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	// Start the server in a goroutine.
	go func() {
		if err := app.Listen(":" + cfg.ServerPort); err != nil {
			log.Printf("Server error: %v", err)
		}
	}()
	// Block until a signal is received.
	<-quit
	log.Println("Shutdown signal received, gracefully shutting down...")

	// --- Signal the WebSocket Hub to shut down ---
	// By calling cancel(), the ctx.Done() channel in wsHub.Run() will be closed,
	// allowing the Hub to exit its loop cleanly.
	cancel()

	// Attempt a graceful shutdown.
	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exiting")
}
