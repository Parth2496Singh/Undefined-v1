# Velzion Staging Environment Runbook

This runbook details the setup process for preparing the target AWS EC2 instance to receive automated CI/CD deployments of the Velzion BYOC Platform via GitHub Actions.

## 1. EC2 Prerequisites

1.  **Launch an EC2 Instance:**
    *   **AMI:** Ubuntu 24.04 LTS (or 22.04 LTS)
    *   **Instance Type:** `t3.medium` (or `t3.small` minimum)
    *   **Storage:** 30GB gp3

2.  **Security Group Ports (Inbound):**
    *   `22` (SSH) - Your IP & GitHub Actions IPs
    *   `80` / `443` (HTTP/HTTPS) - Frontend UI
    *   `8081` - Auth Service (Callbacks)
    *   `8082` - Velzard Service (Telemetry)
    *   `8083` - Zegion Service (Webhooks)
    *   `8084` - Telemetry Service & Admin Control Plane
    *   `3000` - Grafana Dashboard
    *   `9090` - Prometheus Server

3.  **Install Docker & Docker Compose:**
    SSH into the instance and run:
    ```bash
    sudo apt-get update
    sudo apt-get install -y docker.io docker-compose-v2 git
    sudo usermod -aG docker $USER
    sudo mkdir -p /opt/velzion-staging
    sudo chown $USER:$USER /opt/velzion-staging
    ```

4.  **Initial Clone:**
    ```bash
    git clone https://github.com/<your-org>/velzion-v2.git /opt/velzion-staging
    ```

## 2. GitHub Actions Secrets

To enable the `.github/workflows/deploy-staging.yml` pipeline, configure the following **Repository Secrets** in GitHub (Settings > Secrets and variables > Actions):

*   `EC2_HOST`: The Public IP or DNS of your EC2 instance (e.g., `54.123.45.67`).
*   `EC2_USERNAME`: The SSH user (e.g., `ubuntu`).
*   `EC2_SSH_KEY`: The raw private SSH key (`.pem`) authorized to access the EC2 instance.

## 3. GitHub OAuth App Reconfiguration

The local OAuth callbacks will fail in staging. You must update your GitHub OAuth App settings to point to the EC2 instance:

1.  Go to GitHub Developer Settings > OAuth Apps.
2.  Update **Homepage URL:** `http://<EC2_PUBLIC_IP>` (or your domain).
3.  Update **Authorization callback URL:** `http://<EC2_PUBLIC_IP>:8081/api/auth/github/callback`

## 4. GitHub Webhooks (Zegion)

To ensure PR ephemeral environments trigger automatically on staging:

1.  Go to your target repository's **Settings > Webhooks**.
2.  Add a webhook pointing to: `http://<EC2_PUBLIC_IP>:8083/api/zegion/webhook/github`
3.  **Content type:** `application/json`
4.  **Secret:** Match the `GITHUB_WEBHOOK_SECRET` in your `.env`.
5.  **Events:** Select "Pull requests".

## 5. Environment Variables (.env)

Ensure you populate a `.env` file at `/opt/velzion-staging/.env` on the EC2 instance containing your PostgreSQL database URL, AWS credentials (or attach an IAM Instance Profile), JWT Secret, and GitHub client credentials.
