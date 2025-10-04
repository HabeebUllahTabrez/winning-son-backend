package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"winsonin/internal/db"
	"winsonin/internal/handlers"
	mw "winsonin/internal/middleware"
	"winsonin/internal/services"
)

func mustGetenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}

func main() {
	_ = godotenv.Load()

	var logger *zap.Logger
	var err error
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "development" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic("failed to initialize logger")
	}
	defer logger.Sync() // flushes buffer, if any

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		logger.Warn("DATABASE_URL not set; API will run but DB is unavailable")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		logger.Fatal("JWT_SECRET is required")
	}

	encryptionKey := os.Getenv("ENCRYPTION_KEY")
	if encryptionKey == "" {
		logger.Fatal("ENCRYPTION_KEY is required (must be 32 bytes)")
	}
	if len(encryptionKey) != 32 {
		logger.Fatal("ENCRYPTION_KEY must be exactly 32 bytes")
	}

	blindIndexKey := os.Getenv("BLIND_INDEX_KEY")
	if blindIndexKey == "" {
		logger.Fatal("BLIND_INDEX_KEY is required (must be 32 bytes)")
	}
	if len(blindIndexKey) != 32 {
		logger.Fatal("BLIND_INDEX_KEY must be exactly 32 bytes")
	}

	port := mustGetenv("PORT", "8080")

	var dbConn *sqlx.DB
	if databaseURL != "" {
		var err error
		dbConn, err = sqlx.Open("pgx", databaseURL)
		if err != nil {
			logger.Fatal("failed to open db", zap.Error(err))
		}
		dbConn.SetMaxOpenConns(10)
		dbConn.SetConnMaxLifetime(2 * time.Hour)
		if err = dbConn.Ping(); err != nil {
			logger.Fatal("failed to ping db", zap.Error(err))
		}
		if err := db.RunMigrations(dbConn); err != nil {
			logger.Fatal("failed migrations", zap.Error(err))
		}
	}

	// Initialize encryption service
	encSvc, err := services.NewEncryptionService([]byte(encryptionKey), []byte(blindIndexKey))
	if err != nil {
		logger.Fatal("failed to initialize encryption service", zap.Error(err))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	if appEnv == "development" {
		r.Use(middleware.Logger)
	} else {
		r.Use(mw.StructuredLogger(logger))
	}

	// Get allowed origins from environment or use defaults
	allowedOrigins := []string{"http://localhost:3000", "http://localhost:5173", "http://127.0.0.1:3000", "http://127.0.0.1:5173"}
	if origins := os.Getenv("ALLOWED_ORIGINS"); origins != "" {
		// Split by comma if multiple origins are provided
		allowedOrigins = []string{}
		for _, origin := range strings.Split(origins, ",") {
			allowedOrigins = append(allowedOrigins, strings.TrimSpace(origin))
		}
	}

	// CORS middleware configuration
	corsMiddleware := cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Requested-With", "Origin"},
		ExposedHeaders:   []string{"Link", "Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	})

	r.Use(corsMiddleware)

	// r.Get("/error", func(w http.ResponseWriter, r *http.Request) {
	// 	logger.Error("this is a test error")
	// 	http.Error(w, "internal server error", http.StatusInternalServerError)
	// })

	authHandler := handlers.NewAuthHandler(dbConn, []byte(jwtSecret), encSvc)
	journalHandler := handlers.NewJournalHandler(dbConn, encSvc)
	dashboardHandler := handlers.NewDashboardHandler(dbConn, encSvc)
	userHandler := handlers.NewUserHandler(dbConn, encSvc)
	adminHandler := handlers.NewAdminHandler(dbConn)
	migrateHandler := handlers.NewMigrateHandler(dbConn)
	authMW := mw.NewAuthMiddleware([]byte(jwtSecret))

	routeAPI := func(api chi.Router) {
		api.Post("/auth/signup", authHandler.Signup)
		api.Post("/auth/login", authHandler.Login)
		api.Group(func(pr chi.Router) {
			pr.Use(authMW.RequireAuth)
			pr.Post("/migrate", migrateHandler.MigrateData)
			pr.Post("/journal", journalHandler.UpsertEntry)
			pr.Delete("/journal", journalHandler.Delete)
			pr.Get("/journal", journalHandler.List)
			pr.Get("/dashboard", dashboardHandler.Get)
			pr.Get("/dashboard/submission-history", dashboardHandler.GetSubmissionHistory)
			pr.Get("/me", userHandler.GetMe)
			pr.Put("/me", userHandler.UpdateMe)
			pr.Get("/admin/overview", adminHandler.Overview)
		})
	}

	r.Route("/api", routeAPI)

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	srv := &http.Server{Addr: ":" + port, Handler: r}
	go func() {
		logger.Info("server starting", zap.String("addr", ":"+port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("shutdown initiated")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	logger.Info("server stopped")
}
