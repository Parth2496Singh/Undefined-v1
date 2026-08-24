package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/velzion/velzion-v2/services/telemetry/internal/config"
	"github.com/velzion/velzion-v2/services/telemetry/internal/handler"
	"github.com/velzion/velzion-v2/services/telemetry/internal/service"
	"github.com/velzion/velzion-v2/shared/telemetry"
)

func AdminMiddleware(secret string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || len(authHeader) < 8 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		tokenStr := authHeader[7:]
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method")
			}
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			fmt.Printf("JWT Parse Error: %v\n", err)
			http.Error(w, "Unauthorized: invalid token", http.StatusUnauthorized)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			fmt.Printf("JWT Claims Error\n")
			http.Error(w, "Unauthorized: invalid claims", http.StatusUnauthorized)
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
	cfg := config.Load()

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	agg := service.NewAggregator(pool)
	h := handler.NewTelemetryHandler(agg)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/telemetry/ingest", h.HandleIngest)
	mux.HandleFunc("/api/telemetry/summary", h.HandleSummary)
	mux.HandleFunc("/api/admin/deployments", AdminMiddleware(cfg.JWTSecret, h.HandleAdminDeployments))
	mux.HandleFunc("/api/admin/system/reconcile", AdminMiddleware(cfg.JWTSecret, h.HandleAdminReconcile))
	mux.HandleFunc("/api/admin/system/summary", AdminMiddleware(cfg.JWTSecret, h.HandleAdminSystemSummary))
	mux.HandleFunc("/api/admin/system/summary", AdminMiddleware(cfg.JWTSecret, h.HandleAdminSystemSummary))
	mux.Handle("/metrics", promhttp.Handler())

	wrappedMux := telemetry.MetricsMiddleware("telemetry")(mux)

	log.Printf("Telemetry service starting on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, wrappedMux); err != nil {
		log.Fatal(err)
	}
}
