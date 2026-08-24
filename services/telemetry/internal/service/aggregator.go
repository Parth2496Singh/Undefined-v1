package service

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Aggregator struct {
	db        *pgxpool.Pool
	startTime time.Time
}

func NewAggregator(db *pgxpool.Pool) *Aggregator {
	return &Aggregator{
		db:        db,
		startTime: time.Now(),
	}
}

func (a *Aggregator) GetSummary(ctx context.Context) (map[string]interface{}, error) {
	var activeDeployments, activeZegions, totalDeployments, totalZegions int

	// Resilient querying. Using separate queries to tolerate individual table locks or absence
	err := a.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployments WHERE status IN ('PROVISIONING', 'RUNNING')").Scan(&activeDeployments)
	if err != nil {
		activeDeployments = 0
	}

	err = a.db.QueryRow(ctx, "SELECT COUNT(*) FROM ephemeral_environments WHERE status IN ('PROVISIONING', 'BUILT')").Scan(&activeZegions)
	if err != nil {
		activeZegions = 0
	}

	err = a.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployments").Scan(&totalDeployments)
	if err != nil {
		totalDeployments = 0
	}

	err = a.db.QueryRow(ctx, "SELECT COUNT(*) FROM ephemeral_environments").Scan(&totalZegions)
	if err != nil {
		totalZegions = 0
	}

	return map[string]interface{}{
		"active_nodes":     activeDeployments + activeZegions,
		"total_deployments": totalDeployments + totalZegions,
		"uptime_seconds":   int(time.Since(a.startTime).Seconds()),
	}, nil
}

func (a *Aggregator) GetAllDeployments(ctx context.Context) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	
	rows, err := a.db.Query(ctx, "SELECT id, github_repo_url, branch, status, 'velzard' as engine, started_at, destroyed_at FROM deployments")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, repo, contextVal, status, engine string
			var startedAt, destroyedAt interface{}
			if err := rows.Scan(&id, &repo, &contextVal, &status, &engine, &startedAt, &destroyedAt); err == nil {
				results = append(results, map[string]interface{}{"id": id, "repo": repo, "context": contextVal, "status": status, "engine": engine, "started_at": startedAt, "destroyed_at": destroyedAt})
			}
		}
	}

	rows2, err := a.db.Query(ctx, "SELECT id::text, github_repo_url, pr_number::text, status, 'zegion' as engine, started_at, destroyed_at FROM ephemeral_environments")
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var id, repo, contextVal, status, engine string
			var startedAt, destroyedAt interface{}
			if err := rows2.Scan(&id, &repo, &contextVal, &status, &engine, &startedAt, &destroyedAt); err == nil {
				results = append(results, map[string]interface{}{"id": id, "repo": repo, "context": "PR #" + contextVal, "status": status, "engine": engine, "started_at": startedAt, "destroyed_at": destroyedAt})
			}
		}
	}

	return results, nil
}

func (a *Aggregator) GetSystemSummary(ctx context.Context) (map[string]interface{}, error) {
	var totalUsers, totalSuccessful, totalFailed, totalTerminations int
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM users").Scan(&totalUsers)
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployments WHERE status IN ('RUNNING', 'PROVISIONING')").Scan(&totalSuccessful)
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployments WHERE status = 'FAILED'").Scan(&totalFailed)
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM deployments WHERE status = 'DESTROYED'").Scan(&totalTerminations)
	
	var zRunning, zFailed, zTerminated int
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM ephemeral_environments WHERE status IN ('BUILT', 'PROVISIONING')").Scan(&zRunning)
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM ephemeral_environments WHERE status = 'FAILED'").Scan(&zFailed)
	a.db.QueryRow(ctx, "SELECT COUNT(*) FROM ephemeral_environments WHERE status = 'TERMINATED'").Scan(&zTerminated)

	return map[string]interface{}{
		"total_users": totalUsers,
		"total_successful_deployments": totalSuccessful + zRunning,
		"total_failed_deployments": totalFailed + zFailed,
		"total_successful_terminations": totalTerminations + zTerminated,
	}, nil
}

func (a *Aggregator) Reconcile(ctx context.Context) (int, error) {
	var total int64
	tag1, err := a.db.Exec(ctx, "UPDATE deployments SET status = 'FAILED' WHERE status IN ('PROVISIONING', 'DESTROYING')")
	if err == nil {
		total += tag1.RowsAffected()
	}
	tag2, err := a.db.Exec(ctx, "UPDATE ephemeral_environments SET status = 'FAILED' WHERE status IN ('PROVISIONING', 'DESTROYING')")
	if err == nil {
		total += tag2.RowsAffected()
	}
	return int(total), nil
}
