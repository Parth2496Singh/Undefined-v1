# Velzion BYOC Platform (Go Refactor) - Specifications

## Overview
Velzion is an open-source BYOC (Bring Your Own Cloud) control plane that provisions AWS infrastructure. We are migrating from a Django monolith to a modern Go Workspace Monorepo.

## Architectural Rules
1. **Target Language:** Go 1.27+
2. **Monorepo:** Go Workspaces (`go.work`)
3. **Database:** PostgreSQL 16 via `pgx/v5` (no GORM).
4. **Routing:** `http.ServeMux` (Go 1.22+ standard library).
5. **AWS SDK:** Go v2 (`github.com/aws/aws-sdk-go-v2`).
6. **Terraform:** 1.15.x target compatibility.

## Target Architecture (Blueprint)

🌐 PLATFORM INFRASTRUCTURE
1. Kong API Gateway 
2. NATS JetStream 

🔐 CORE MICROSERVICES
3. Frontend Service (React App - already exists)
4. Auth Service (GitHub OAuth, JWT, RBAC, AWS IAM linking)
5. Velzard Service (Production EC2 engine, Docker Compose deployments)
6. Zegion Service (Ephemeral PR-based environments, Spot instances)
7. Diablo Service (EKS + GitOps deployments via Helm/ArgoCD)
8. Observability Service (OpenTelemetry, Prometheus, Loki, Tempo, Grafana)
9. Notification Service
10. Audit Service 

⚙️ EXECUTION LAYER
11. Infrastructure Worker (Executes Terraform/AWS provisioning, decoupled from HTTP)

## API Contracts: Auth Service
| Endpoint | Method | Description | Request Body | Response |
|----------|--------|-------------|--------------|----------|
| `/api/auth/github/login` | GET | Initiates OAuth flow, returns GitHub login URL | None | `{ "login_url": "..." }` |
| `/api/auth/github/callback` | POST | Exchanges code for token, syncs user info & repos, issues JWT | `{ "code": "..." }` | `{ "message": "...", "token": "jwt_string", "user": {...}, "repos": [...] }` |
| `/api/auth/iam-role` | POST | Binds an AWS IAM Role ARN to the authenticated user | `{ "arn": "..." }` | `{ "message": "..." }` |
| `/api/auth/logout` | POST | Invalidates session (clears cookie/token) | None | `{ "message": "..." }` |
| `/api/admin/system/flush` | DELETE | Factory Reset Database | None | `{ "message": "..." }` |

## API Contracts: Telemetry Service (Port 8084)
| Endpoint | Method | Description | Request Body | Response |
|----------|--------|-------------|--------------|----------|
| `/api/telemetry/summary` | GET | Public overview of platform metrics | None | `{ "active_nodes": ..., "total_deployments": ..., "uptime_seconds": ... }` |
| `/api/admin/system/summary` | GET | Admin overview of deployment metrics | None | `{ "total_users": ..., "total_successful_deployments": ... }` |
| `/api/admin/deployments` | GET | Global admin view of all fleets | None | Array of unified deployments |
| `/api/admin/system/reconcile` | POST | Fails all ghost PROVISIONING states | None | `{ "flushed_count": ... }` |

## API Contracts: Velzard Service
| Endpoint | Method | Description | Request Body | Response |
|----------|--------|-------------|--------------|----------|
| `/api/velzard/deployments` | GET | List Velzard Deployments | None | `[{ "id": "...", "status": "...", "github_repo_url": "...", "branch": "...", "instance_type": "...", "volume_size": 30, "error_logs": "...", "created_at": "..." }]` |
| `/api/velzard/deploy` | POST | Verifies `velzion.yaml`, assumes IAM role, starts async TF apply | `{ "repo_url": "...", "branch": "main", "instance_type": "t3.small", "volume_size": 30 }` | `{ "message": "...", "deployment_id": "uuid" }` |
| `/api/velzard/destroy/{id}` | POST | Starts async TF destroy | None | `{ "message": "..." }` |
| `/api/velzard/webhook/{id}` | PATCH | Webhook from EC2 to update status | `{ "status": "..." }` | `{ "message": "..." }` |
| `/api/velzard/telemetry/{id}` | POST | OTLP ingestion from EC2 sidecar | OTLP JSON Payload | `{ "status": "..." }` |

## API Contracts: Zegion Service
| Endpoint | Method | Description | Request Body | Response |
|----------|--------|-------------|--------------|----------|
| `/api/zegion/environments` | GET | List Zegion Environments | None | `[{ "id": "...", "status": "...", "github_repo_url": "...", "pr_number": 12, "error_logs": "...", "created_at": "..." }]` |
| `/api/zegion/webhook/github` | POST | Parses GitHub PR events (`opened`, `closed`), enqueues `ProvisionJob` or `DestroyJob` | GitHub Webhook JSON | `200 OK` |
| `/api/zegion/webhook/ec2` | POST | Receives internal boot callbacks from Spot instances | `{ "status": "BUILT", "instance_id": "..." }` | `200 OK` |
| `/api/zegion/terminate/{id}` | POST | Manual GUI-based termination of a Zegion environment | None | `{ "message": "..." }` |

## Service Port Map
| Service | Default Port | Environment Variable |
|---------|--------------|----------------------|
| Auth    | 8081         | `AUTH_PORT`          |
| Velzard | 8082         | `VELZARD_PORT`       |
| Zegion  | 8083         | `ZEGION_PORT`        |
| Telemetry| 8084        | `TELEMETRY_PORT`     |
