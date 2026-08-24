package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/velzion/velzion-v2/services/zegion/internal/config"
	"github.com/velzion/velzion-v2/services/zegion/internal/repository"
	"github.com/velzion/velzion-v2/services/zegion/internal/service"
	"github.com/velzion/velzion-v2/services/zegion/internal/worker"
)

type ZegionHandler struct {
	cfg      *config.Config
	repo     *repository.ZegionRepository
	awsSvc   *service.AWSService
	jobQueue *worker.JobQueue
}

func NewZegionHandler(cfg *config.Config, repo *repository.ZegionRepository, awsSvc *service.AWSService, jq *worker.JobQueue) *ZegionHandler {
	return &ZegionHandler{
		cfg:      cfg,
		repo:     repo,
		awsSvc:   awsSvc,
		jobQueue: jq,
	}
}

func (h *ZegionHandler) verifyGitHubSignature(payload []byte, signature string) bool {
	if signature == "" || h.cfg.GithubWebhookSecret == "" {
		return false
	}
	
	parts := strings.SplitN(signature, "=", 2)
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	mac := hmac.New(sha256.New, []byte(h.cfg.GithubWebhookSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(parts[1]), []byte(expectedMAC))
}

func (h *ZegionHandler) GithubWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.verifyGitHubSignature(body, signature) {
		http.Error(w, "Unauthorized signature", http.StatusForbidden)
		return
	}

	var payload struct {
		Action     string `json:"action"`
		Repository struct {
			CloneURL string `json:"clone_url"`
			Owner    struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
		PullRequest struct {
			Number int `json:"number"`
		} `json:"pull_request"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if payload.PullRequest.Number == 0 || payload.Repository.CloneURL == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	userCtx, err := h.repo.GetUserByUsername(r.Context(), payload.Repository.Owner.Login)
	if err != nil || userCtx.IAMRoleARN == "" {
		fmt.Printf("Webhook received but no bound user IAM Role found for %s\n", payload.Repository.Owner.Login)
		w.WriteHeader(http.StatusOK)
		return
	}

	action := payload.Action
	var jobType worker.JobType
	var status string

	if action == "opened" || action == "synchronize" || action == "reopened" {
		jobType = worker.JobProvision
		status = "PROVISIONING"
	} else if action == "closed" {
		jobType = worker.JobDestroy
		status = "DESTROYING"
	} else {
		w.WriteHeader(http.StatusOK)
		return
	}

	envID, err := h.repo.UpsertEnvironment(r.Context(), userCtx.ID, payload.Repository.CloneURL, payload.PullRequest.Number, status)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "ZegionSession-"+userCtx.ID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), envID, "FAILED")
		fmt.Printf("Failed to assume STS role in Zegion: %v\n", err)
		w.WriteHeader(http.StatusOK) // Webhook succeeds, but orchestration fails
		return
	}

	job := worker.ZegionJob{
		Type:     jobType,
		EnvID:    envID,
		RepoURL:  payload.Repository.CloneURL,
		PRNumber: payload.PullRequest.Number,
		AWSCreds: creds,
		GithubAccessToken: userCtx.GithubAccessToken,
	}

	h.jobQueue.Push(job)

	w.WriteHeader(http.StatusOK)
}

func (h *ZegionHandler) EC2Webhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		RepoURL    string `json:"repo_url"`
		PRNumber   string `json:"pr_number"`
		InstanceID string `json:"instance_id"`
		Status     string `json:"status"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	var prNum int
	fmt.Sscanf(payload.PRNumber, "%d", &prNum)

	if payload.Status == "BUILT" {
		h.repo.MarkBuilt(r.Context(), prNum, payload.RepoURL, payload.InstanceID)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ZegionHandler) Terminate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	envID := strings.TrimPrefix(r.URL.Path, "/api/zegion/terminate/")
	if envID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("user_id").(string)

	env, err := h.repo.GetEnvironment(r.Context(), envID)
	if err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	if env["status"] == "DESTROYING" || env["status"] == "DESTROYED" {
		http.Error(w, "Already destroyed", http.StatusBadRequest)
		return
	}

	h.repo.UpdateStatus(r.Context(), envID, "DESTROYING")

	userCtx, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil || userCtx.IAMRoleARN == "" {
		http.Error(w, "User IAM Role not found", http.StatusForbidden)
		return
	}

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "ZegionSession-"+userID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), envID, "DESTROY_FAILED")
		http.Error(w, fmt.Sprintf("Failed to assume STS role: %v", err), http.StatusForbidden)
		return
	}

	job := worker.ZegionJob{
		Type:     worker.JobDestroy,
		EnvID:    envID,
		RepoURL:  env["github_repo_url"].(string),
		PRNumber: env["pr_number"].(int),
		AWSCreds: creds,
		GithubAccessToken: userCtx.GithubAccessToken,
	}

	h.jobQueue.Push(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Destruction initiated"})
}

func (h *ZegionHandler) AdminTerminate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	envID := strings.TrimPrefix(r.URL.Path, "/api/admin/zegion/terminate/")
	if envID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	env, err := h.repo.GetEnvironment(r.Context(), envID)
	if err != nil {
		http.Error(w, "Environment not found", http.StatusNotFound)
		return
	}

	if env["status"] == "DESTROYING" || env["status"] == "DESTROYED" || env["status"] == "TERMINATED" || env["status"] == "FAILED" || env["status"] == "SLEEPING" {
		http.Error(w, "Already destroying, destroyed, or failed", http.StatusBadRequest)
		return
	}

	h.repo.UpdateStatus(r.Context(), envID, "DESTROYING")

	userID := env["user_id"].(string)
	userCtx, err := h.repo.GetUserByID(r.Context(), userID)
	if err != nil || userCtx.IAMRoleARN == "" {
		http.Error(w, "User IAM Role not found", http.StatusForbidden)
		return
	}

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "ZegionAdminSession-"+userID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), envID, "DESTROY_FAILED")
		http.Error(w, fmt.Sprintf("Failed to assume STS role: %v", err), http.StatusForbidden)
		return
	}

	job := worker.ZegionJob{
		Type:     worker.JobDestroy,
		EnvID:    envID,
		RepoURL:  env["github_repo_url"].(string),
		PRNumber: env["pr_number"].(int),
		AWSCreds: creds,
		GithubAccessToken: userCtx.GithubAccessToken,
	}

	h.jobQueue.Push(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Destruction initiated by Admin"})
}

func (h *ZegionHandler) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("user_id").(string)
	envs, err := h.repo.GetEnvironmentsByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(envs)
}
