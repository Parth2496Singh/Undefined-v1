package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DeployRepository struct {
	db *pgxpool.Pool
}

func NewDeployRepository(db *pgxpool.Pool) *DeployRepository {
	return &DeployRepository{db: db}
}

type UserContext struct {
	IAMRoleARN        string
	GithubAccessToken string
}

func (r *DeployRepository) GetUserContext(ctx context.Context, userID string) (*UserContext, error) {
	query := `SELECT iam_role_arn, github_access_token FROM users WHERE id = $1`
	var u UserContext
	err := r.db.QueryRow(ctx, query, userID).Scan(&u.IAMRoleARN, &u.GithubAccessToken)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user context: %w", err)
	}
	return &u, nil
}

func (r *DeployRepository) CreateDeployment(ctx context.Context, userID, repoURL, branch, instanceType string, volSize int, snapshot string) (string, error) {
	query := `
		INSERT INTO deployments (user_id, github_repo_url, branch, instance_type, volume_size, config_snapshot, status)
		VALUES ($1, $2, $3, $4, $5, $6, 'PROVISIONING')
		RETURNING id
	`
	var id string
	err := r.db.QueryRow(ctx, query, userID, repoURL, branch, instanceType, volSize, snapshot).Scan(&id)
	return id, err
}

func (r *DeployRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE deployments SET status = $1, updated_at = CURRENT_TIMESTAMP`
	if status == "RUNNING" {
		query += `, started_at = CURRENT_TIMESTAMP`
	} else if status == "DESTROYED" {
		query += `, destroyed_at = CURRENT_TIMESTAMP`
	}
	query += ` WHERE id = $2`
	_, err := r.db.Exec(ctx, query, status, id)
	return err
}

func (r *DeployRepository) UpdateStatusAndError(ctx context.Context, id, status, errorLogs string) error {
	query := `UPDATE deployments SET status = $1, error_logs = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err := r.db.Exec(ctx, query, status, errorLogs, id)
	return err
}

func (r *DeployRepository) RecoverOrphanedDeployments(ctx context.Context) error {
	query := `UPDATE deployments SET status = 'FAILED', error_logs = 'Server crashed during execution.' WHERE status IN ('PROVISIONING', 'DESTROYING')`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *DeployRepository) UpdateWebhookDetails(ctx context.Context, id, status, instanceID, ip string) error {
	query := `
		UPDATE deployments 
		SET status = COALESCE(NULLIF($1, ''), status),
		    aws_instance_id = COALESCE(NULLIF($2, ''), aws_instance_id),
		    elastic_ip = COALESCE(NULLIF($3, ''), elastic_ip),
		    updated_at = CURRENT_TIMESTAMP`
	
	if status == "RUNNING" {
		query += `, started_at = CURRENT_TIMESTAMP`
	} else if status == "DESTROYED" {
		query += `, destroyed_at = CURRENT_TIMESTAMP`
	}
	
	query += ` WHERE id = $4`
	_, err := r.db.Exec(ctx, query, status, instanceID, ip, id)
	return err
}

func (r *DeployRepository) GetDeployment(ctx context.Context, id string) (map[string]interface{}, error) {
	query := `SELECT id, status, github_repo_url, branch, instance_type, volume_size, telemetry_history FROM deployments WHERE id = $1`
	var (
		depID, status, repoURL, branch, iType string
		volSize                               int
		telemetryHistory                      string
	)
	err := r.db.QueryRow(ctx, query, id).Scan(&depID, &status, &repoURL, &branch, &iType, &volSize, &telemetryHistory)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":                depID,
		"status":            status,
		"github_repo_url":   repoURL,
		"branch":            branch,
		"instance_type":     iType,
		"volume_size":       volSize,
		"telemetry_history": telemetryHistory,
	}, nil
}

func (r *DeployRepository) GetDeploymentsByUserID(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `SELECT id, status, github_repo_url, branch, instance_type, volume_size, error_logs, created_at FROM deployments WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []map[string]interface{}
	for rows.Next() {
		var (
			depID, status, repoURL, branch, iType string
			volSize                               int
			errorLogs                             *string
			createdAt                             interface{}
		)
		if err := rows.Scan(&depID, &status, &repoURL, &branch, &iType, &volSize, &errorLogs, &createdAt); err != nil {
			return nil, err
		}

		el := ""
		if errorLogs != nil {
			el = *errorLogs
		}

		deps = append(deps, map[string]interface{}{
			"id":              depID,
			"status":          status,
			"github_repo_url": repoURL,
			"branch":          branch,
			"instance_type":   iType,
			"volume_size":     volSize,
			"error_logs":      el,
			"created_at":      createdAt,
		})
	}
	return deps, nil
}
