package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

type User struct {
	ID                string
	GithubID          string
	Username          string
	Email             string
	AvatarURL         string
	GithubAccessToken string
	AllowedRepos      []string
	IAMRoleARN        *string
	IsAdmin           bool
}

func (r *UserRepository) UpsertGitHubUser(ctx context.Context, u *User) (*User, error) {
	query := `
		INSERT INTO users (github_id, username, email, avatar_url, github_access_token, allowed_repos)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (github_id) DO UPDATE SET
			username = EXCLUDED.username,
			email = EXCLUDED.email,
			avatar_url = EXCLUDED.avatar_url,
			github_access_token = EXCLUDED.github_access_token,
			allowed_repos = EXCLUDED.allowed_repos,
			updated_at = CURRENT_TIMESTAMP
		RETURNING id, github_id, username, email, avatar_url, github_access_token, allowed_repos, iam_role_arn, is_admin
	`
	
	var user User
	err := r.db.QueryRow(ctx, query, u.GithubID, u.Username, u.Email, u.AvatarURL, u.GithubAccessToken, u.AllowedRepos).Scan(
		&user.ID, &user.GithubID, &user.Username, &user.Email, &user.AvatarURL, &user.GithubAccessToken, &user.AllowedRepos, &user.IAMRoleARN, &user.IsAdmin,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) UpdateIAMRole(ctx context.Context, userID string, iamRoleARN string) error {
	query := `UPDATE users SET iam_role_arn = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	tag, err := r.db.Exec(ctx, query, iamRoleARN, userID)
	if err != nil {
		return fmt.Errorf("failed to update IAM role: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *UserRepository) GetUserByID(ctx context.Context, userID string) (*User, error) {
	query := `SELECT id, github_id, username, email, avatar_url, github_access_token, allowed_repos, iam_role_arn, is_admin FROM users WHERE id = $1`
	var user User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.GithubID, &user.Username, &user.Email, &user.AvatarURL, &user.GithubAccessToken, &user.AllowedRepos, &user.IAMRoleARN, &user.IsAdmin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, err
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) DeleteUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, "DELETE FROM deployments WHERE user_id = $1", userID)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, "DELETE FROM ephemeral_environments WHERE user_id = $1", userID)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func (r *UserRepository) FactoryReset(ctx context.Context, adminUserID string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, "DELETE FROM deployments"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM ephemeral_environments"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, "DELETE FROM users WHERE id != $1", adminUserID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *UserRepository) GetAllUsers(ctx context.Context) ([]User, error) {
	query := `SELECT id, github_id, username, email, avatar_url, github_access_token, allowed_repos, iam_role_arn, is_admin FROM users`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.GithubID, &u.Username, &u.Email, &u.AvatarURL, &u.GithubAccessToken, &u.AllowedRepos, &u.IAMRoleARN, &u.IsAdmin); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, nil
}
