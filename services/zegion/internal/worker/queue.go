package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/velzion/velzion-v2/services/zegion/internal/repository"
	"github.com/velzion/velzion-v2/services/zegion/internal/service"
	"github.com/velzion/velzion-v2/shared/telemetry"
)

type JobType string

const (
	JobProvision JobType = "PROVISION"
	JobDestroy   JobType = "DESTROY"
)

type ZegionJob struct {
	Type     JobType
	EnvID    string
	RepoURL  string
	PRNumber int
	AWSCreds          *service.TempCreds
	GithubAccessToken string
}

type JobQueue struct {
	jobs chan ZegionJob
	repo *repository.ZegionRepository
}

func NewJobQueue(repo *repository.ZegionRepository, maxQueueSize int) *JobQueue {
	return &JobQueue{
		jobs: make(chan ZegionJob, maxQueueSize),
		repo: repo,
	}
}

func (q *JobQueue) Push(job ZegionJob) {
	q.jobs <- job
}

func (q *JobQueue) StartWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go q.workerLoop(i)
	}
}

func (q *JobQueue) workerLoop(workerID int) {
	log.Printf("Zegion Worker %d starting...", workerID)
	for job := range q.jobs {
		log.Printf("Worker %d processing job %s for env %s", workerID, job.Type, job.EnvID)
		
		start := time.Now()
		publicIP, err := q.executeTerraform(job)
		duration := time.Since(start).Seconds()
		telemetry.DeploymentDuration.WithLabelValues("zegion").Observe(duration)

		if err != nil {
			log.Printf("Worker %d failed job %s: %v", workerID, job.EnvID, err)
			telemetry.DeploymentsTotal.WithLabelValues("zegion", "failed").Inc()
			status := "FAILED"
			if job.Type == JobDestroy {
				status = "DESTROY_FAILED"
			}
			_ = q.repo.UpdateStatusAndError(context.Background(), job.EnvID, status, err.Error())
		} else {
			log.Printf("Worker %d completed job %s", workerID, job.EnvID)
			telemetry.DeploymentsTotal.WithLabelValues("zegion", "success").Inc()
			
			if job.Type == JobDestroy {
				telemetry.ActiveEnvironments.WithLabelValues("zegion").Dec()
				_ = q.repo.UpdateStatusAndError(context.Background(), job.EnvID, "DESTROYED", "")
				if job.GithubAccessToken != "" {
					_ = service.PostPRComment(context.Background(), job.RepoURL, job.PRNumber, "✅ **Velzion Zegion**: Ephemeral PR environment successfully destroyed.", job.GithubAccessToken)
				}
			} else {
				telemetry.ActiveEnvironments.WithLabelValues("zegion").Inc()
				if job.GithubAccessToken != "" {
					msg := fmt.Sprintf("🚀 **Velzion Zegion**: Ephemeral PR environment successfully provisioned.\n\nSpot Instance is spinning up at: http://%s\nTTL hibernation will trigger automatically.", publicIP)
					_ = service.PostPRComment(context.Background(), job.RepoURL, job.PRNumber, msg, job.GithubAccessToken)
				}
			}
		}
	}
}

func (q *JobQueue) executeTerraform(job ZegionJob) (string, error) {
	tmpDir := fmt.Sprintf("/tmp/zegion_%s", job.EnvID)
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return "", err
	}

	// Copy terraform file
	tfSource := "terraform/main.tf"
	tfDest := filepath.Join(tmpDir, "main.tf")
	if job.Type == JobProvision {
		b, err := os.ReadFile(tfSource)
		if err != nil {
			return "", fmt.Errorf("read tf source failed: %w", err)
		}
		if err := os.WriteFile(tfDest, b, 0644); err != nil {
			return "", fmt.Errorf("write tf dest failed: %w", err)
		}
	}

	env := os.Environ()
	if job.AWSCreds != nil {
		env = append(env,
			"AWS_ACCESS_KEY_ID="+job.AWSCreds.AccessKeyID,
			"AWS_SECRET_ACCESS_KEY="+job.AWSCreds.SecretAccessKey,
			"AWS_SESSION_TOKEN="+job.AWSCreds.SessionToken,
		)
	}

	// Init
	initCmd := exec.CommandContext(context.Background(), "terraform", "init")
	initCmd.Dir = tmpDir
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("terraform init failed:\n%s", string(out))
	}

	action := "apply"
	if job.Type == JobDestroy {
		action = "destroy"
	}

	cmd := exec.CommandContext(context.Background(), "terraform", action, "-auto-approve",
		fmt.Sprintf("-var=repo_url=%s", job.RepoURL),
		fmt.Sprintf("-var=pr_number=%d", job.PRNumber),
	)
	cmd.Dir = tmpDir
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("terraform %s failed:\n%s", action, string(out))
	}
	
	var publicIP string
	if action == "apply" {
		outCmd := exec.CommandContext(context.Background(), "terraform", "output", "-json")
		outCmd.Dir = tmpDir
		outCmd.Env = env
		if outputData, err := outCmd.Output(); err == nil {
			var tfOutputs map[string]struct{ Value string }
			if err := json.Unmarshal(outputData, &tfOutputs); err == nil {
				if ip, ok := tfOutputs["public_ip"]; ok {
					publicIP = ip.Value
				}
			}
		}
	}

	if job.Type == JobDestroy {
		os.RemoveAll(tmpDir)
	}

	return publicIP, nil
}
