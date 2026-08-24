# Implementation Log

## Phase 0: Scaffolding and Archiving
- Archived legacy Django files (`backend/`, `docker-compose.yml`) into `.legacy_django/`.
- Updated `.docs/` with new Target Architecture and strict guidelines.
- Generated root `go.work` and initialized microservices structure:
  - `shared/`
  - `services/auth/`
  - `services/velzard/`
  - `services/zegion/`
  - `services/diablo/`
  - `services/observability/`
  - `services/notification/`
  - `services/audit/`
  - `workers/infra-worker/`
- Created root `docker-compose.yml` restricted to PostgreSQL 16.
- Created root `Makefile` for basic operations.

## Phase 1: Auth Service (Go Refactor)
- Scaffolded `services/auth` microservice with `cmd`, `internal/config`, `internal/handler`, `internal/repository`, and `internal/service`.
- Migrated legacy Django `UserProfile` model to pure SQL schema (`migrations/001_create_users_table.sql`) including `allowed_repos` JSONB firewall logic.
- Implemented `jackc/pgx/v5/pgxpool` connection and raw SQL `UserRepository` (no ORM) with explicit error handling (e.g. `pgx.ErrNoRows`).
- Re-implemented GitHub OAuth flow and decoupled it using standard library `http.ServeMux`.
- Replaced Django session cookies with standard `golang-jwt/jwt/v5` for robust microservice boundaries.

## Phase 2: Velzard Engine (Go Refactor)
- Scaffolded `services/velzard` microservice with JWT boundary protection.
- Migrated AWS STS logic to Go `aws-sdk-go-v2`.
- Migrated Python subprocess orchestration to an **In-Memory Job Queue** (`DeployJob` channel).
- Worker goroutines now execute `terraform init` and `apply` synchronously using `os/exec` via `context.Background()` to prevent orphan deployments upon HTTP disconnects.
- Created `deployments` database schema mimicking legacy models.

## Phase 3: Zegion Engine (Go Refactor)
- Scaffolded `services/zegion` microservice with GitHub Webhook parsing and HMAC signature validation (`X-Hub-Signature-256`).
- Refactored schema (`migrations/003_create_ephemeral_envs_table.sql`) to resolve foreign keys dynamically across domains (linking `ephemeral_environments` to `users(id)`).
- Webhook parses `repository.owner.login` and fetches `iam_role_arn` securely via cross-query.
- Adapted In-Memory Job Queue to handle `ProvisionJob` and `DestroyJob` operations for AWS Spot Instances using `aws_spot_instance_request` and CNCF Buildpacks (`pack build`).

## Phase 4: Frontend Integration & OpenAPI Contract
- Engineered unified `.docs/openapi.yaml` contract outlining schemas and Bearer Auth requirements.
- Configured Vite reverse proxy to dynamically forward React API requests seamlessly to `localhost:8081` (Auth), `localhost:8082` (Velzard), and `localhost:8083` (Zegion).
- Designed `frontend/src/api/index.js` Axios singleton featuring automatic JWT Bearer injection via interceptors.
- Developed the `App.jsx` React dashboard encompassing Auth IAM binding, Velzard Production provisioning, and Zegion Ephemeral Preview kill switches.

## Phase 5: Enterprise Feature Restoration
- Crafted `.docs/velzion-trust.yaml` CloudFormation template exposing an `AssumeRole` payload scoped securely for the Velzion Orchestrator.
- Integrated automated GitHub PR Commenting into Zegion (`services/zegion/internal/service/github_comment.go`). Comments trigger safely inside worker threads post-provision.
- Augmented Auth Service with `GET /api/auth/github/repos` which leverages token passthrough to fetch the user's active repositories directly into the UI.
- Revamped `frontend/src/App.jsx` with an enterprise-grade dark-themed UX, interactive form dropdowns, and pulsing color-coded status badges.

## Phase 6: Observability, Recovery, & CI/CD
- Migrated schemas to include `error_logs TEXT` for Terraform output persistence.
- Refactored Go worker `executeTerraform` loops to forcefully capture `cmd.CombinedOutput()` and inject it via `UpdateStatusAndError()`.
- Implemented boot-time State Machine Crash Recovery in `velzard/cmd/api/main.go` and `zegion/cmd/api/main.go` to purge ghost processes left in `PROVISIONING` or `DESTROYING`.
- Built Velzard Contract Pre-flight Validation, fetching `velzion.yaml` securely via the user's vaulted `GithubAccessToken` prior to triggering AWS STS.
- Augmented Zegion workers to extract `terraform output -json` and automatically comment the live Elastic IP directly on the GitHub Pull Request.
- Completely refactored `frontend/src/App.jsx` into a Multi-Page Application using `react-router-dom`, featuring a dedicated logs modal and sidebar layout.
- Engineered a zero-downtime GitHub Actions staging pipeline (`deploy-staging.yml`) and comprehensive `STAGING_RUNBOOK.md`.

## Phase 8: Telemetry & Observability
- Added `prometheus` and `grafana` to `docker-compose.yml`, provisioning `prometheus:9090` as the default Grafana data source.
- Scaffolded a dedicated `services/telemetry` microservice (port 8084) to compute aggregated cluster health from PostgreSQL.
- Authored a `shared/telemetry` Go module, configuring `prometheus/client_golang` vectors (`http_requests_total`, `velzion_deployments_total`, `velzion_active_environments`, etc.).
- Embedded `MetricsMiddleware` across the Go stack (`auth`, `velzard`, `zegion`) and dynamically hooked business metrics into the in-memory queue Terraform loops.
- Updated `Makefile` to concurrently boot the telemetry engine.
- Augmented the React SPA with a new `Platform Health` dashboard route, polling the telemetry gateway every 5 seconds.

## Phase 9: Multi-Tenant Admin Control Plane
- Upgraded `users` schema with `is_admin` BOOLEAN, backing the global `AdminMiddleware` RBAC across Go microservices (`auth`, `velzard`, `zegion`, `telemetry`).
- Scaffolded protected `/api/admin/*` endpoints to extract global matrices of Users, Velzard fleets, and Zegion instances.
- Engineered a "Reconcile State" database control mechanism in the Telemetry service to forcefully flush stuck orchestration jobs.
- Implemented `AdminDashboard` in `frontend/src/App.jsx` complete with an embedded Grafana `<iframe>` view via `GF_SECURITY_ALLOW_EMBEDDING=true`, and cross-engine `FORCE KILL` actions.

## Phase 10: Admin Panel Data Density & Revocation Controls
- Enhanced `telemetry` aggregator to fetch branch context and engine type (`velzard` or `zegion`) for the Fleet Control UI.
- Implemented `DELETE /api/admin/users/{id}` in `auth` service using cascading `DELETE` SQL transaction blocks to gracefully wipe user orchestration history.
- Built a comprehensive Grafana Dashboard JSON schema (`velzion-overview.json`) featuring active cluster gauges, deployment donuts, latency heatmaps, and memory allocation time series.
- Augmented React Admin Dashboard with engine-colored badges, context tracking, a glaring red `Revoke Access` button, and an expansive 800px Kiosk-mode Grafana embed.

## Phase 11: Audit Trails & State Machine Hardening
- Authored SQL Migrations injecting `started_at` and `destroyed_at` timestamps across `deployments` and `ephemeral_environments`.
- Refactored Go orchestration repositories (`velzard`, `zegion`) to dynamically set `CURRENT_TIMESTAMP` on exact state transitions (`RUNNING`, `BUILT`, `DESTROYED`).
- Fortified `AdminTerminate` endpoints with HTTP `400 Bad Request` Idempotency Guardrails (`FAILED`, `SLEEPING`, `DESTROYING`).
- Orchestrated the "Nuclear Option" via `DELETE /api/admin/system/flush` in the `auth` service, wrapping a total cross-domain database wipe in a singular ACID transaction (`BEGIN ... COMMIT`).
- Upgraded the React UI with 4 native metric cards, time-localized Audit Trails (Deploy Time, Uptime calculations), and dynamic conditional UI rendering logic for destructive actions.
