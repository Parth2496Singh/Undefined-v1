package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/velzion/velzion-v2/services/auth/internal/config"
	"github.com/velzion/velzion-v2/services/auth/internal/repository"
	"github.com/velzion/velzion-v2/services/auth/internal/service"
)

type AuthHandler struct {
	cfg       *config.Config
	repo      *repository.UserRepository
	ghService *service.GitHubService
	jwtSvc    *service.JWTService
}

func NewAuthHandler(cfg *config.Config, repo *repository.UserRepository, ghService *service.GitHubService, jwtSvc *service.JWTService) *AuthHandler {
	return &AuthHandler{
		cfg:       cfg,
		repo:      repo,
		ghService: ghService,
		jwtSvc:    jwtSvc,
	}
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	redirectURI := fmt.Sprintf("http://localhost:%s/api/auth/github/callback", h.cfg.Port)
	url := fmt.Sprintf("https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user,user:email,repo", h.cfg.GitHubClientID, redirectURI)

	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

type CallbackRequest struct {
	Code string `json:"code"`
}

func (h *AuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	token, err := h.ghService.ExchangeCodeForToken(code)
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}

	ghUser, err := h.ghService.FetchUserProfile(token)
	if err != nil {
		http.Error(w, "Failed to fetch user profile", http.StatusInternalServerError)
		return
	}

	repos, err := h.ghService.FetchUserRepos(token)
	if err != nil {
		fmt.Printf("Warning: failed to fetch repos: %v\n", err)
		repos = []string{}
	}

	userReq := &repository.User{
		GithubID:          strconv.Itoa(ghUser.ID),
		Username:          ghUser.Login,
		Email:             ghUser.Email,
		AvatarURL:         ghUser.AvatarURL,
		GithubAccessToken: token,
		AllowedRepos:      repos,
	}

	user, err := h.repo.UpsertGitHubUser(r.Context(), userReq)
	if err != nil {
		http.Error(w, "Failed to save user", http.StatusInternalServerError)
		return
	}

	jwtToken, err := h.jwtSvc.GenerateToken(user.ID, user.IsAdmin)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	redirectURL := fmt.Sprintf("%s/?token=%s", h.cfg.FrontendURL, jwtToken)
	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

type IAMRoleRequest struct {
	ARN string `json:"arn"`
}

func (h *AuthHandler) BindIAMRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("user_id").(string)
	
	var req IAMRoleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	arn := strings.TrimSpace(req.ARN)
	if arn == "" || !strings.HasPrefix(arn, "arn:aws:iam::") || strings.Contains(arn, " ") {
		http.Error(w, "Invalid ARN format. Must start with arn:aws:iam:: and contain no spaces.", http.StatusBadRequest)
		return
	}

	err := h.repo.UpdateIAMRole(r.Context(), userID, arn)
	if err != nil {
		http.Error(w, "Failed to bind IAM role", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "IAM Role bound to workspace successfully.",
	})
}

func (h *AuthHandler) ListUserRepos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("user_id").(string)
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil || user.GithubAccessToken == "" {
		http.Error(w, "User or access token not found", http.StatusForbidden)
		return
	}

	repos, err := h.ghService.FetchUserRepos(user.GithubAccessToken)
	if err != nil {
		http.Error(w, "Failed to fetch repositories", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(repos)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Logged out successfully"})
}

func (h *AuthHandler) GetAdminUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	users, err := h.repo.GetAllUsers(r.Context())
	if err != nil {
		http.Error(w, "Failed to get users", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}

func (h *AuthHandler) AdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := strings.TrimPrefix(r.URL.Path, "/api/admin/users/")
	if userID == "" {
		http.Error(w, "Missing user id", http.StatusBadRequest)
		return
	}
	err := h.repo.DeleteUser(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "User deleted successfully"}`))
}

func (h *AuthHandler) AdminFactoryReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	userID := r.Context().Value("user_id").(string)
	
	err := h.repo.FactoryReset(r.Context(), userID)
	if err != nil {
		http.Error(w, "Failed to flush system", http.StatusInternalServerError)
		return
	}
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"message": "System flushed successfully"}`))
}

func AuthMiddleware(jwtSvc *service.JWTService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, isAdmin, err := jwtSvc.ValidateToken(token)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "user_id", userID)
		ctx = context.WithValue(ctx, "is_admin", isAdmin)
		next(w, r.WithContext(ctx))
	}
}

func AdminMiddleware(jwtSvc *service.JWTService, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		userID, isAdmin, err := jwtSvc.ValidateToken(token)
		if err != nil {
			http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
			return
		}
		
		if !isAdmin {
			http.Error(w, "Forbidden: Admin access required", http.StatusForbidden)
			return
		}

		ctx := r.Context()
		ctx = context.WithValue(ctx, "user_id", userID)
		ctx = context.WithValue(ctx, "is_admin", isAdmin)
		next(w, r.WithContext(ctx))
	}
}
