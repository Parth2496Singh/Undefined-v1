# Velzion BYOC Platform
> Enterprise-grade "Bring Your Own Cloud" Orchestrator

![Go](https://img.shields.io/badge/go-%2300ADD8.svg?style=for-the-badge&logo=go&logoColor=white)
![React](https://img.shields.io/badge/react-%2320232a.svg?style=for-the-badge&logo=react&logoColor=%2361DAFB)
![Terraform](https://img.shields.io/badge/terraform-%235835CC.svg?style=for-the-badge&logo=terraform&logoColor=white)
![AWS](https://img.shields.io/badge/AWS-%23FF9900.svg?style=for-the-badge&logo=amazon-aws&logoColor=white)
![PostgreSQL](https://img.shields.io/badge/postgresql-%23316192.svg?style=for-the-badge&logo=postgresql&logoColor=white)
![Grafana](https://img.shields.io/badge/grafana-%23F46800.svg?style=for-the-badge&logo=grafana&logoColor=white)
![Prometheus](https://img.shields.io/badge/Prometheus-E6522C?style=for-the-badge&logo=Prometheus&logoColor=white)

Velzion is a powerful control plane designed to eliminate the paradox of modern cloud development: giving developers the zero-configuration PaaS experience they crave, while allowing organizations to retain absolute ownership, deep database isolation, and raw infrastructure cost-control in their own AWS accounts.

By leveraging an architecture of Go microservices and Terraform, Velzion connects directly to your AWS environment via secure IAM STS Role Delegation and orchestrates both Production (EC2) and Ephemeral PR Preview (Spot Instance) infrastructure dynamically based on a simple `velzion.yaml` contract.

## 🚀 Key Features

*   **1-Click CloudFormation Trust Model:** Securely delegate AWS orchestration privileges to the Velzion control plane without handing over static access keys.
*   **Velzard Production Engine:** A dedicated Go microservice utilizing an in-memory job queue to orchestrate stable, highly-available EC2 infrastructure deployments.
*   **Zegion Ephemeral Engine:** A real-time GitHub webhook listener that automatically spins up cost-effective AWS Spot Instances for Pull Requests, commenting the live URL on the PR and destroying it when closed.
*   **The Admin Control Plane:** Global fleet management powered by strict RBAC. Monitor cross-engine deployments, execute force kills, and trigger ACID-compliant Database Factory Resets wrapped in PostgreSQL transactions.
*   **LGTM Telemetry:** Integrated Prometheus and Grafana observability stack with native time-series metrics. Monitor real-time memory allocation, deployment latency, and system active nodes natively in the React dashboard.

## 🏗️ System Architecture

```mermaid
graph TD
    UI["Vite (React SPA)"] -->|Reverse Proxy| Auth["Auth Service (:8081)"]
    UI -->|Reverse Proxy| Velzard["Velzard Engine (:8082)"]
    UI -->|Reverse Proxy| Zegion["Zegion Engine (:8083)"]
    UI -->|Reverse Proxy| Telemetry["Telemetry Service (:8084)"]
    
    Auth --> DB[("PostgreSQL 16")]
    Velzard --> DB
    Zegion --> DB
    Telemetry --> DB
    
    Velzard -->|AssumeRole| AWS["AWS STS"]
    Zegion -->|AssumeRole| AWS
    
    AWS -->|Provision| EC2["EC2 Instances (Spot / On-Demand)"]
    EC2 -->|Heartbeat| Telemetry
    
    Prometheus -->|Scrape| Auth
    Prometheus -->|Scrape| Velzard
    Prometheus -->|Scrape| Zegion
    Prometheus -->|Scrape| Telemetry
    
    Grafana --> Prometheus
```

The core backend consists of 4 discrete Go microservices:
*   `services/auth` - Handles GitHub OAuth, JWT issuance, IAM Role bindings, and destructive System Flushes.
*   `services/velzard` - The primary Terraform runner orchestrating persistent Production deployments.
*   `services/zegion` - The webhook-driven Terraform runner generating ephemeral PR environments.
*   `services/telemetry` - Aggregates system metrics, computes timestamps, and exposes `http_requests_total` vectors.

## ⚙️ Setup Guide: Local Development

### 1. Prerequisites
- **Go 1.27+**
- **Docker & Docker Compose**
- **Node.js 20+**
- **GitHub OAuth App** (Set Homepage URL to `http://localhost:5173` (or your EC2 Public IP) and Callback URL to `http://localhost:8081/api/auth/github/callback` (or `http://<EC2_PUBLIC_IP>:8081/api/auth/github/callback`))

### 2. Environment Configuration
Create a `.env` file in the root directory:
```env
DATABASE_URL=postgres://velzion_user:velzion_pass@localhost:5432/velzion_dev?sslmode=disable
JWT_SECRET=supersecret123
GITHUB_CLIENT_ID=your_oauth_client_id
GITHUB_CLIENT_SECRET=your_oauth_client_secret
GITHUB_WEBHOOK_SECRET=your_webhook_secret
AWS_REGION=us-east-1
```

### 3. Booting the Platform
We provide a root `Makefile` to instantly spin up the Postgres database, Prometheus/Grafana LGTM stack, the Vite Frontend, and all 4 Go microservices concurrently.
```bash
make dev
```
Navigate to `http://localhost:5173` to access the Velzion Control Plane.

---

## 📖 Documentation & Runbooks

For an in-depth understanding of the system, API contracts, and CI/CD staging, refer to our comprehensive `.docs/` directory:

*   [**TESTING_RUNBOOK.md**](./.docs/TESTING_RUNBOOK.md): Guide for verifying deployments and microservice interaction.
*   [**STAGING_RUNBOOK.md**](./.docs/STAGING_RUNBOOK.md): The precise steps for deploying this platform to an EC2 CI/CD staging environment via GitHub Actions.
*   [**SPECS.md**](./.docs/SPECS.md): Exhaustive API contracts, routing architecture, and data payloads.
*   [**IMPLEMENTATION_LOG.md**](./.docs/IMPLEMENTATION_LOG.md): The historical log of architecture decisions and scaling milestones.
*   [**DECISIONS.md**](./.docs/DECISIONS.md): Architectural logic outlining our transition to pure SQL and isolated job queues.

## 🛡️ License
MIT License. See `LICENSE` for more information.
