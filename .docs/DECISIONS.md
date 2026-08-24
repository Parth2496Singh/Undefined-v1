# Architectural Decisions

## Decision 1: Language & Architecture
- **Context:** Moving away from a monolithic Django application to support scalability and decoupled execution.
- **Decision:** Go 1.27+ using Go Workspaces for a monorepo setup.

## Decision 2: Database Strategy
- **Decision:** Use PostgreSQL 16 with `pgx/v5`.
- **Reasoning:** High performance, raw SQL usage without the overhead of heavy ORMs like GORM.

## Decision 3: Routing
- **Decision:** `net/http` standard library `ServeMux` (Go 1.22+ features).
- **Reasoning:** Avoids dependency on external routing frameworks (like Gin or Echo) by leveraging standard library improvements.

## Decision 4: Orchestration Concurrency Model
- **Context:** Terraform `os/exec` subprocesses must run in the background without blocking the HTTP server.
- **Decision:** Use an In-Memory Job Queue with buffered channels (`chan DeployJob`) and a fixed worker pool, initialized with `context.Background()`.
- **Reasoning:** Spawning naked goroutines inside HTTP handlers is risky because if the HTTP request is canceled, the context terminates, and the Terraform subprocess becomes orphaned. A dedicated worker pool solves this.

## Decision 5: Crash Recovery & Observability
- **Context:** In-memory job queues lose state if the Go server crashes midway through a Terraform apply/destroy operation.
- **Decision:** Implement a Boot-Time State Machine Reconciliation strategy and `cmd.CombinedOutput()` trapping.
- **Reasoning:** Upon server start, the engine queries the database and forcibly fails any orchestrations lingering in active states (`PROVISIONING`, `DESTROYING`). By aggressively logging Terraform outputs, we guarantee visibility even if the worker panics.

## Decision 6: Frontend Architecture
- **Context:** Transitioning from a monolith to a microservice control plane requires a structured frontend.
- **Decision:** Shifted to a Multi-Page Application (SPA) layout utilizing `react-router-dom` with Tailwind CSS v4.
- **Reasoning:** Allows distinct DevOps capabilities (Production vs. Ephemeral) to be isolated by domain while sharing identity context and API singletons from a global state.

## Decision 7: Admin Control Plane & Telemetry
- **Context:** An enterprise system requires cross-tenant observability and data management.
- **Decision:** Scaffolded an independent `services/telemetry` microservice and integrated the Prometheus/Grafana stack natively into the control plane via iframe embeds. RBAC is enforced globally via a pure Go middleware intercepting `is_admin` claims in JWTs.
- **Reasoning:** Decoupling telemetry from the core orchestration workers (`velzard`, `zegion`) ensures that heavy aggregation queries and PromQL polling do not starve the Terraform worker threads.
