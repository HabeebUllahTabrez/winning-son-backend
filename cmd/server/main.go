package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
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
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	authHandler := handlers.NewAuthHandler(dbConn, []byte(jwtSecret))
	progressHandler := handlers.NewProgressHandler(dbConn)
	journalHandler := handlers.NewJournalHandler(dbConn)
	authMW := mw.NewAuthMiddleware([]byte(jwtSecret))

	r.Route("/api", func(api chi.Router) {
		api.Post("/auth/signup", authHandler.Signup)
		api.Post("/auth/login", authHandler.Login)
		api.Group(func(pr chi.Router) {
			pr.Use(authMW.RequireAuth)
			pr.Get("/progress", progressHandler.GetSummary)
			pr.Post("/progress", progressHandler.AddProgress)
			pr.Get("/progress/report", progressHandler.GetReport)
			pr.Post("/journal", journalHandler.AddToday)
			pr.Get("/journal", journalHandler.List)
		})
	})

	staticDir := mustGetenv("STATIC_DIR", "../frontend/out")
	spa := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath := filepath.Clean(r.URL.Path)
		rel := strings.TrimPrefix(requestedPath, "/")
		if rel == "" {
			rel = "index.html"
		}
		candidate := filepath.Join(staticDir, rel)
		if info, err := os.Stat(candidate); err == nil {
			if info.IsDir() {
				index := filepath.Join(candidate, "index.html")
				if _, err := os.Stat(index); err == nil {
					http.ServeFile(w, r, index)
					return
				}
			} else {
				http.ServeFile(w, r, candidate)
				return
			}
		}
		if !strings.Contains(filepath.Base(rel), ".") {
			htmlPath := filepath.Join(staticDir, rel+".html")
			if _, err := os.Stat(htmlPath); err == nil {
				http.ServeFile(w, r, htmlPath)
				return
			}
		}
		http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
	})
	r.Handle("/*", spa)

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
