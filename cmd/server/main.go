package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/joho/godotenv"

	"winsonin/internal/db"
	"winsonin/internal/handlers"
	mw "winsonin/internal/middleware"
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

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		slog.Warn("DATABASE_URL not set; API will run but DB is unavailable")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		slog.Error("JWT_SECRET is required")
		os.Exit(1)
	}

	port := mustGetenv("PORT", "8080")

	var dbConn *sqlx.DB
	var err error
	if databaseURL != "" {
		dbConn, err = sqlx.Open("pgx", databaseURL)
		if err != nil {
			slog.Error("failed to open db", slog.Any("err", err))
			os.Exit(1)
		}
		dbConn.SetMaxOpenConns(10)
		dbConn.SetConnMaxLifetime(2 * time.Hour)
		if err = dbConn.Ping(); err != nil {
			slog.Error("failed to ping db", slog.Any("err", err))
			os.Exit(1)
		}
		if err := db.RunMigrations(dbConn); err != nil {
			slog.Error("failed migrations", slog.Any("err", err))
			os.Exit(1)
		}
	}

	r := chi.NewRouter()

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

	authHandler := handlers.NewAuthHandler(dbConn, []byte(jwtSecret))
	journalHandler := handlers.NewJournalHandler(dbConn)
	dashboardHandler := handlers.NewDashboardHandler(dbConn)
	userHandler := handlers.NewUserHandler(dbConn)
	adminHandler := handlers.NewAdminHandler(dbConn)
	migrateHandler := handlers.NewMigrateHandler(dbConn)
	authMW := mw.NewAuthMiddleware([]byte(jwtSecret))

	routeAPI := func(api chi.Router) {
		api.Post("/auth/signup", authHandler.Signup)
		api.Post("/auth/login", authHandler.Login)
		api.Post("/migrate", migrateHandler.MigrateData)
		api.Group(func(pr chi.Router) {
			pr.Use(authMW.RequireAuth)
			pr.Post("/journal", journalHandler.UpsertEntry)
			pr.Delete("/journal", journalHandler.Delete)
			pr.Get("/journal", journalHandler.List)
			pr.Get("/dashboard", dashboardHandler.Get)
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
		slog.Info("server starting", slog.String("addr", ":"+port))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", slog.Any("err", err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutdown initiated")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	slog.Info("server stopped")
}
