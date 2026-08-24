package worker

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/velzion/velzion-v2/services/velzard/internal/repository"
	"github.com/velzion/velzion-v2/services/velzard/internal/service"
	"github.com/velzion/velzion-v2/shared/telemetry"
)

type JobType string

const (
	JobDeploy  JobType = "DEPLOY"
	JobDestroy JobType = "DESTROY"
)

type DeployJob struct {
	Type         JobType
	DeploymentID string
	RepoURL      string
	Branch       string
	InstanceType string
	VolumeSize   int
	AWSCreds     *service.TempCreds
	GithubAccessToken string
}

type JobQueue struct {
	jobs chan DeployJob
	repo *repository.DeployRepository
}

func NewJobQueue(repo *repository.DeployRepository, maxQueueSize int) *JobQueue {
	return &JobQueue{
		jobs: make(chan DeployJob, maxQueueSize),
		repo: repo,
	}
}

func (q *JobQueue) Push(job DeployJob) {
	q.jobs <- job
}

func (q *JobQueue) StartWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go q.workerLoop(i)
	}
}

func (q *JobQueue) workerLoop(workerID int) {
	log.Printf("Worker %d starting...", workerID)
	for job := range q.jobs {
		log.Printf("Worker %d processing job %s for deployment %s", workerID, job.Type, job.DeploymentID)
		
		start := time.Now()
		err := q.executeTerraform(job)
		duration := time.Since(start).Seconds()
		telemetry.DeploymentDuration.WithLabelValues("velzard").Observe(duration)

		if err != nil {
			log.Printf("Worker %d failed job %s: %v", workerID, job.DeploymentID, err)
			telemetry.DeploymentsTotal.WithLabelValues("velzard", "failed").Inc()
			status := "FAILED"
			if job.Type == JobDestroy {
				status = "DESTROY_FAILED"
			}
			_ = q.repo.UpdateStatusAndError(context.Background(), job.DeploymentID, status, err.Error())
		} else {
			log.Printf("Worker %d completed job %s", workerID, job.DeploymentID)
			telemetry.DeploymentsTotal.WithLabelValues("velzard", "success").Inc()
			
			if job.Type == JobDeploy {
				telemetry.ActiveEnvironments.WithLabelValues("velzard").Inc()
			} else if job.Type == JobDestroy {
				telemetry.ActiveEnvironments.WithLabelValues("velzard").Dec()
				_ = q.repo.UpdateStatusAndError(context.Background(), job.DeploymentID, "DESTROYED", "")
			}
		}
	}
}

func (q *JobQueue) executeTerraform(job DeployJob) error {
	tmpDir := fmt.Sprintf("/tmp/velzard_%s", job.DeploymentID)
	err := os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return err
	}

	// Copy terraform file
	tfSource := "terraform/velzard_main.tf"
	tfDest := filepath.Join(tmpDir, "main.tf")
	if job.Type == JobDeploy {
		// Phase 6.3: Contract Pre-flight validation
		parts := strings.Split(job.RepoURL, "/")
		if len(parts) >= 2 {
			repoName := strings.TrimSuffix(parts[len(parts)-1], ".git")
			owner := parts[len(parts)-2]
			rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/velzion.yaml", owner, repoName, job.Branch)
			
			req, err := http.NewRequest("GET", rawURL, nil)
			if err == nil {
				if job.GithubAccessToken != "" {
					req.Header.Set("Authorization", "token "+job.GithubAccessToken)
				}
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil || resp.StatusCode != 200 {
					if resp != nil {
						resp.Body.Close()
					}
					return fmt.Errorf("velzion.yaml contract not found in repository root")
				}
				resp.Body.Close()
			}
		}

		b, err := os.ReadFile(tfSource)
		if err != nil {
			return fmt.Errorf("read tf source failed: %w", err)
		}
		if err := os.WriteFile(tfDest, b, 0644); err != nil {
			return fmt.Errorf("write tf dest failed: %w", err)
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

	// Init (Using background context so it survives HTTP termination)
	initCmd := exec.CommandContext(context.Background(), "terraform", "init")
	initCmd.Dir = tmpDir
	initCmd.Env = env
	if out, err := initCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform init failed:\n%s", string(out))
	}

	action := "apply"
	if job.Type == JobDestroy {
		action = "destroy"
	}

	// Apply/Destroy
	cmd := exec.CommandContext(context.Background(), "terraform", action, "-auto-approve",
		fmt.Sprintf("-var=repo_url=%s", job.RepoURL),
		fmt.Sprintf("-var=branch=%s", job.Branch),
		fmt.Sprintf("-var=deployment_id=%s", job.DeploymentID),
		fmt.Sprintf("-var=instance_type=%s", job.InstanceType),
		fmt.Sprintf("-var=volume_size=%d", job.VolumeSize),
		"-var=backend_url=http://localhost:8082",
	)
	cmd.Dir = tmpDir
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terraform %s failed:\n%s", action, string(out))
	}

	if job.Type == JobDestroy {
		os.RemoveAll(tmpDir)
	}

	return nil
}
