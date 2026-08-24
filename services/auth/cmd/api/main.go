package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/velzion/velzion-v2/services/auth/internal/config"
	"github.com/velzion/velzion-v2/services/auth/internal/handler"
	"github.com/velzion/velzion-v2/services/auth/internal/repository"
	"github.com/velzion/velzion-v2/services/auth/internal/service"
	"github.com/velzion/velzion-v2/shared/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.LoadConfig()

	// Init DB
	dbpool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}
	log.Println("Connected to PostgreSQL successfully.")

	// Init layers
	repo := repository.NewUserRepository(dbpool)
	ghSvc := service.NewGitHubService(cfg.GitHubClientID, cfg.GitHubClientSecret)
	jwtSvc := service.NewJWTService(cfg.JWTSecret)
	
	authHandler := handler.NewAuthHandler(cfg, repo, ghSvc, jwtSvc)

	// Routing
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/github/login", authHandler.Login)
	mux.HandleFunc("/api/auth/github/callback", authHandler.Callback)
	mux.HandleFunc("/api/auth/github/repos", handler.AuthMiddleware(jwtSvc, authHandler.ListUserRepos))
	mux.HandleFunc("/api/auth/logout", authHandler.Logout)
	mux.HandleFunc("/api/auth/iam-role", handler.AuthMiddleware(jwtSvc, authHandler.BindIAMRole))
	mux.HandleFunc("/api/admin/users", handler.AdminMiddleware(jwtSvc, authHandler.GetAdminUsers))
	mux.HandleFunc("/api/admin/users/", handler.AdminMiddleware(jwtSvc, authHandler.AdminDeleteUser))
	mux.HandleFunc("/api/admin/system/flush", handler.AdminMiddleware(jwtSvc, authHandler.AdminFactoryReset))
	mux.Handle("/metrics", promhttp.Handler())

	wrappedMux := telemetry.MetricsMiddleware("auth")(mux)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: wrappedMux,
	}

	go func() {
		fmt.Printf("Auth service starting on port %s\n", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen: %s\n", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}
	log.Println("Server exiting")
}
