package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/velzion/velzion-v2/services/zegion/internal/config"
	"github.com/velzion/velzion-v2/services/zegion/internal/handler"
	"github.com/velzion/velzion-v2/services/zegion/internal/repository"
	"github.com/velzion/velzion-v2/services/zegion/internal/service"
	"github.com/velzion/velzion-v2/services/zegion/internal/worker"
	"github.com/velzion/velzion-v2/shared/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func AuthMiddleware(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
			return
		}

		userID, ok := claims["user_id"].(string)
		if !ok {
			http.Error(w, "Unauthorized: user_id missing", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", userID)
		next(w, r.WithContext(ctx))
	}
}

func AdminMiddleware(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		isAdmin, ok := claims["is_admin"].(bool)
		if !ok || !isAdmin {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		next(w, r)
	}
}

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

	repo := repository.NewZegionRepository(dbpool)
	
	if err := repo.RecoverOrphanedEnvironments(context.Background()); err != nil {
		log.Printf("Warning: Failed to recover orphaned environments: %v\n", err)
	}

	awsSvc := service.NewAWSService(cfg.AWSRegion)
	
	jq := worker.NewJobQueue(repo, 100)
	jq.StartWorkers(5)

	zh := handler.NewZegionHandler(cfg, repo, awsSvc, jq)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/zegion/webhook/github", zh.GithubWebhook)
	mux.HandleFunc("/api/zegion/webhook/ec2", zh.EC2Webhook)
	mux.HandleFunc("/api/zegion/environments", AuthMiddleware(cfg.JWTSecret, zh.ListEnvironments))
	mux.HandleFunc("/api/zegion/terminate/", AuthMiddleware(cfg.JWTSecret, zh.Terminate))
	mux.HandleFunc("/api/admin/zegion/terminate/", AdminMiddleware(cfg.JWTSecret, zh.AdminTerminate))
	mux.Handle("/metrics", promhttp.Handler())

	wrappedMux := telemetry.MetricsMiddleware("zegion")(mux)

	srv := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: wrappedMux,
	}

	go func() {
		log.Printf("Zegion service starting on port %s", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Listen: %s", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Zegion server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
