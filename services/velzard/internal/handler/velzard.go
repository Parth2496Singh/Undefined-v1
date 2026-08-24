package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/velzion/velzion-v2/services/velzard/internal/config"
	"github.com/velzion/velzion-v2/services/velzard/internal/repository"
	"github.com/velzion/velzion-v2/services/velzard/internal/service"
	"github.com/velzion/velzion-v2/services/velzard/internal/worker"
)

type VelzardHandler struct {
	cfg      *config.Config
	repo     *repository.DeployRepository
	awsSvc   *service.AWSService
	jobQueue *worker.JobQueue
}

func NewVelzardHandler(cfg *config.Config, repo *repository.DeployRepository, awsSvc *service.AWSService, jq *worker.JobQueue) *VelzardHandler {
	return &VelzardHandler{
		cfg:      cfg,
		repo:     repo,
		awsSvc:   awsSvc,
		jobQueue: jq,
	}
}

type DeployRequest struct {
	RepoURL      string `json:"repo_url"`
	Branch       string `json:"branch"`
	InstanceType string `json:"instance_type"`
	VolumeSize   int    `json:"volume_size"`
}

func (h *VelzardHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("user_id").(string)

	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.InstanceType == "" {
		req.InstanceType = "t3.small"
	}
	if req.VolumeSize == 0 {
		req.VolumeSize = 30
	}

	userCtx, err := h.repo.GetUserContext(r.Context(), userID)
	if err != nil || userCtx.IAMRoleARN == "" {
		http.Error(w, "User IAM Role not bound. Cannot deploy to AWS.", http.StatusForbidden)
		return
	}

	depID, err := h.repo.CreateDeployment(r.Context(), userID, req.RepoURL, req.Branch, req.InstanceType, req.VolumeSize, "{}")
	if err != nil {
		http.Error(w, "Failed to create deployment record", http.StatusInternalServerError)
		return
	}

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "VelzionSession-"+userID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), depID, "FAILED")
		http.Error(w, fmt.Sprintf("Failed to assume STS role: %v", err), http.StatusForbidden)
		return
	}

	job := worker.DeployJob{
		Type:         worker.JobDeploy,
		DeploymentID: depID,
		RepoURL:      req.RepoURL,
		Branch:       req.Branch,
		InstanceType: req.InstanceType,
		VolumeSize:   req.VolumeSize,
		AWSCreds:     creds,
		GithubAccessToken: userCtx.GithubAccessToken,
	}

	h.jobQueue.Push(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Deployment orchestrating", "deployment_id": depID})
}

func (h *VelzardHandler) Destroy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	depID := strings.TrimPrefix(r.URL.Path, "/api/velzard/destroy/")
	if depID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("user_id").(string)
	userCtx, err := h.repo.GetUserContext(r.Context(), userID)
	if err != nil || userCtx.IAMRoleARN == "" {
		http.Error(w, "User IAM Role not found", http.StatusForbidden)
		return
	}

	dep, err := h.repo.GetDeployment(r.Context(), depID)
	if err != nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	if dep["status"] == "DESTROYING" || dep["status"] == "DESTROYED" {
		http.Error(w, "Already destroying or destroyed", http.StatusBadRequest)
		return
	}

	h.repo.UpdateStatus(r.Context(), depID, "DESTROYING")

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "VelzionSession-"+userID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), depID, "DESTROY_FAILED")
		http.Error(w, fmt.Sprintf("Failed to assume STS role: %v", err), http.StatusForbidden)
		return
	}

	job := worker.DeployJob{
		Type:         worker.JobDestroy,
		DeploymentID: depID,
		RepoURL:      dep["github_repo_url"].(string),
		Branch:       dep["branch"].(string),
		InstanceType: dep["instance_type"].(string),
		VolumeSize:   dep["volume_size"].(int),
		AWSCreds:     creds,
	}

	h.jobQueue.Push(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Destruction initiated"})
}

func (h *VelzardHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Context().Value("user_id").(string)
	deps, err := h.repo.GetDeploymentsByUserID(r.Context(), userID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(deps)
}

func (h *VelzardHandler) WebhookUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secret := r.Header.Get("x-velzion-secret")
	if secret != h.cfg.TelemetrySecret {
		http.Error(w, "Unauthorized webhook", http.StatusForbidden)
		return
	}

	depID := strings.TrimPrefix(r.URL.Path, "/api/velzard/webhook/")
	
	var req map[string]interface{}
	json.NewDecoder(r.Body).Decode(&req)
	
	status, _ := req["status"].(string)
	if status == "DELETED" || status == "DESTROYED" {
		h.repo.UpdateStatus(r.Context(), depID, "DESTROYED")
		w.WriteHeader(http.StatusOK)
		return
	}

	instanceID, _ := req["aws_instance_id"].(string)
	ip, _ := req["elastic_ip"].(string)

	if status == "" && instanceID != "" && ip != "" {
		status = "RUNNING"
	}

	h.repo.UpdateWebhookDetails(r.Context(), depID, status, instanceID, ip)
	w.WriteHeader(http.StatusOK)
}

func (h *VelzardHandler) Telemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	secret := r.Header.Get("x-velzion-secret")
	if secret != h.cfg.TelemetrySecret {
		http.Error(w, "Unauthorized telemetry", http.StatusForbidden)
		return
	}
	
	// Simplified OTLP placeholder for now.
	// Will translate real OpenTelemetry payload in the future.
	w.WriteHeader(http.StatusOK)
}

func (h *VelzardHandler) AdminTerminate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	depID := strings.TrimPrefix(r.URL.Path, "/api/admin/deployments/")
	depID = strings.TrimSuffix(depID, "/terminate")

	if depID == "" {
		http.Error(w, "Missing ID", http.StatusBadRequest)
		return
	}

	dep, err := h.repo.GetDeployment(r.Context(), depID)
	if err != nil {
		http.Error(w, "Deployment not found", http.StatusNotFound)
		return
	}

	if dep["status"] == "DESTROYING" || dep["status"] == "DESTROYED" || dep["status"] == "FAILED" || dep["status"] == "SLEEPING" {
		http.Error(w, "Already destroying, destroyed, or failed", http.StatusBadRequest)
		return
	}

	userID := r.Context().Value("user_id").(string)
	userCtx, err := h.repo.GetUserContext(r.Context(), userID)
	if err != nil || userCtx.IAMRoleARN == "" {
		http.Error(w, "Admin IAM Role not found", http.StatusForbidden)
		return
	}

	h.repo.UpdateStatus(r.Context(), depID, "DESTROYING")

	creds, err := h.awsSvc.AssumeRole(r.Context(), userCtx.IAMRoleARN, "VelzionAdminSession-"+userID)
	if err != nil {
		h.repo.UpdateStatus(r.Context(), depID, "DESTROY_FAILED")
		http.Error(w, fmt.Sprintf("Failed to assume STS role: %v", err), http.StatusForbidden)
		return
	}

	job := worker.DeployJob{
		Type:         worker.JobDestroy,
		DeploymentID: depID,
		RepoURL:      dep["github_repo_url"].(string),
		Branch:       dep["branch"].(string),
		InstanceType: dep["instance_type"].(string),
		VolumeSize:   dep["volume_size"].(int),
		AWSCreds:     creds,
	}

	h.jobQueue.Push(job)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Destruction initiated by Admin"})
}
