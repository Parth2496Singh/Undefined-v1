package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/velzion/velzion-v2/services/velzard/internal/config"
	"github.com/velzion/velzion-v2/services/velzard/internal/handler"
	"github.com/velzion/velzion-v2/services/velzard/internal/repository"
	"github.com/velzion/velzion-v2/services/velzard/internal/service"
	"github.com/velzion/velzion-v2/services/velzard/internal/worker"
	"github.com/velzion/velzion-v2/shared/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg := config.LoadConfig()

	dbpool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to create connection pool: %v\n", err)
	}
	defer dbpool.Close()

	if err := dbpool.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}
	log.Println("Connected to PostgreSQL successfully.")

	repo := repository.NewDeployRepository(dbpool)
	
	if err := repo.RecoverOrphanedDeployments(context.Background()); err != nil {
		log.Printf("Warning: Failed to recover orphaned deployments: %v\n", err)
	}

	awsSvc := service.NewAWSService(cfg.AWSRegion)
	
	// Start Job Queue (100 capacity, 5 concurrent workers)
	jq := worker.NewJobQueue(repo, 100)
	jq.StartWorkers(5)

	vh := handler.NewVelzardHandler(cfg, repo, awsSvc, jq)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/velzard/deploy", handler.AuthMiddleware(cfg.JWTSecret, vh.Deploy))
	mux.HandleFunc("/api/velzard/deployments", handler.AuthMiddleware(cfg.JWTSecret, vh.ListDeployments))
	mux.HandleFunc("/api/velzard/destroy/", handler.AuthMiddleware(cfg.JWTSecret, vh.Destroy))
	mux.HandleFunc("/api/admin/deployments/", handler.AdminMiddleware(cfg.JWTSecret, vh.AdminTerminate))
	mux.HandleFunc("/api/velzard/webhook/", vh.WebhookUpdate)
	mux.HandleFunc("/api/velzard/telemetry/", vh.Telemetry)
	mux.Handle("/metrics", promhttp.Handler())

	wrappedMux := telemetry.MetricsMiddleware("velzard")(mux)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: wrappedMux,
	}

	go func() {
		log.Printf("Velzard service starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen: %s", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Velzard server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
