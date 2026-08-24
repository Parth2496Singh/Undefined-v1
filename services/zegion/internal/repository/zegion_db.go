package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ZegionRepository struct {
	db *pgxpool.Pool
}

func NewZegionRepository(db *pgxpool.Pool) *ZegionRepository {
	return &ZegionRepository{db: db}
}

type UserContext struct {
	ID                string
	IAMRoleARN        string
	GithubAccessToken string
}

func (r *ZegionRepository) GetUserByUsername(ctx context.Context, username string) (*UserContext, error) {
	query := `SELECT id, iam_role_arn, github_access_token FROM users WHERE username = $1`
	var u UserContext
	err := r.db.QueryRow(ctx, query, username).Scan(&u.ID, &u.IAMRoleARN, &u.GithubAccessToken)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user context: %w", err)
	}
	return &u, nil
}

func (r *ZegionRepository) GetUserByID(ctx context.Context, id string) (*UserContext, error) {
	query := `SELECT id, iam_role_arn, github_access_token FROM users WHERE id = $1`
	var u UserContext
	err := r.db.QueryRow(ctx, query, id).Scan(&u.ID, &u.IAMRoleARN, &u.GithubAccessToken)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (r *ZegionRepository) UpsertEnvironment(ctx context.Context, userID, repoURL string, prNumber int, status string) (string, error) {
	query := `
		INSERT INTO ephemeral_environments (user_id, github_repo_url, pr_number, status)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, github_repo_url, pr_number)
		DO UPDATE SET status = EXCLUDED.status, updated_at = CURRENT_TIMESTAMP
		RETURNING id
	`
	var id string
	err := r.db.QueryRow(ctx, query, userID, repoURL, prNumber, status).Scan(&id)
	return id, err
}

func (r *ZegionRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE ephemeral_environments SET status = $1, updated_at = CURRENT_TIMESTAMP`
	if status == "BUILT" || status == "RUNNING" {
		query += `, started_at = CURRENT_TIMESTAMP`
	} else if status == "DESTROYED" || status == "TERMINATED" {
		query += `, destroyed_at = CURRENT_TIMESTAMP`
	}
	query += ` WHERE id = $2`
	_, err := r.db.Exec(ctx, query, status, id)
	return err
}

func (r *ZegionRepository) UpdateStatusAndError(ctx context.Context, id, status, errorLogs string) error {
	query := `UPDATE ephemeral_environments SET status = $1, error_logs = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $3`
	_, err := r.db.Exec(ctx, query, status, errorLogs, id)
	return err
}

func (r *ZegionRepository) RecoverOrphanedEnvironments(ctx context.Context) error {
	query := `UPDATE ephemeral_environments SET status = 'FAILED', error_logs = 'Server crashed during execution.' WHERE status IN ('PROVISIONING', 'DESTROYING')`
	_, err := r.db.Exec(ctx, query)
	return err
}

func (r *ZegionRepository) MarkBuilt(ctx context.Context, prNumber int, repoURL, instanceID string) error {
	query := `
		UPDATE ephemeral_environments 
		SET status = 'BUILT', instance_id = $1, updated_at = CURRENT_TIMESTAMP, started_at = CURRENT_TIMESTAMP
		WHERE github_repo_url = $2 AND pr_number = $3
	`
	_, err := r.db.Exec(ctx, query, instanceID, repoURL, prNumber)
	return err
}

func (r *ZegionRepository) GetEnvironment(ctx context.Context, id string) (map[string]interface{}, error) {
	query := `SELECT id, status, github_repo_url, pr_number FROM ephemeral_environments WHERE id = $1`
	var (
		envID, status, repoURL string
		prNumber               int
	)
	err := r.db.QueryRow(ctx, query, id).Scan(&envID, &status, &repoURL, &prNumber)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":              envID,
		"status":          status,
		"github_repo_url": repoURL,
		"pr_number":       prNumber,
	}, nil
}

func (r *ZegionRepository) GetEnvironmentsByUserID(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	query := `SELECT id, status, github_repo_url, pr_number, error_logs, created_at FROM ephemeral_environments WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var envs []map[string]interface{}
	for rows.Next() {
		var (
			envID, status, repoURL string
			prNumber               int
			errorLogs              *string
			createdAt              interface{}
		)
		if err := rows.Scan(&envID, &status, &repoURL, &prNumber, &errorLogs, &createdAt); err != nil {
			return nil, err
		}

		el := ""
		if errorLogs != nil {
			el = *errorLogs
		}

		envs = append(envs, map[string]interface{}{
			"id":              envID,
			"status":          status,
			"github_repo_url": repoURL,
			"pr_number":       prNumber,
			"error_logs":      el,
			"created_at":      createdAt,
		})
	}
	return envs, nil
}
