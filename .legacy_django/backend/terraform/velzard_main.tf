variable "repo_url" {
  description = "The target GitHub repository to clone"
  type        = string
}

variable "branch" {
  description = "The target branch or commit hash being deployed"
  type        = string
  default     = "main"
}

variable "deployment_id" {
  description = "The UUID tracking this deployment inside Django"
  type        = string
}

variable "instance_type" {
  description = "The EC2 instance type to provision"
  type        = string
  default     = "t3.small"
}

variable "volume_size" {
  description = "The size of the EBS root volume in GB"
  type        = number
  default     = 30
}

variable "backend_url" {
  description = "The public URL of the Velzion backend server"
  type        = string
}

provider "aws" {
  region = "us-east-1"
}

resource "aws_security_group" "velzard_prod_sg" {
  name_prefix = "velzard_prod_sg_"
  description = "Velzard High-Availability Gateway Security Group"
}

resource "aws_vpc_security_group_ingress_rule" "allow_http_prod" {
  security_group_id = aws_security_group.velzard_prod_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  ip_protocol       = "tcp"
  to_port           = 80
  description       = "Production Public Traffic Gateway"
}

resource "aws_vpc_security_group_ingress_rule" "allow_ssh_prod" {
  security_group_id = aws_security_group.velzard_prod_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 22
  ip_protocol       = "tcp"
  to_port           = 22
  description       = "Emergency Corporate SSH Gateway"
}

resource "aws_vpc_security_group_egress_rule" "allow_all_egress_prod" {
  security_group_id = aws_security_group.velzard_prod_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1" 
  description       = "Allow outbound engine calls to compile dependencies"
}

resource "aws_instance" "velzard_prod_server" {
  ami                    = "ami-04b70fa74e45c3917" # Ubuntu 24.04 LTS
  instance_type          = var.instance_type
  vpc_security_group_ids = [aws_security_group.velzard_prod_sg.id]

  root_block_device {
    volume_size           = var.volume_size
    volume_type           = "gp3"
    delete_on_termination = true
  }

  user_data = <<-EOF
#!/bin/bash
exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1
export HOME=/root

apt-get remove -y docker.io docker-doc docker-compose docker-compose-v2 podman-docker containerd runc || true
apt-get autoremove -y || true

apt-get update -y
apt-get install -y ca-certificates curl gnupg git python3-pip python3-yaml lsb-release

install -m 0755 -d /etc/apt/keyrings
rm -f /etc/apt/keyrings/docker.gpg
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --batch --yes --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

apt-get update -y
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin

systemctl enable docker
systemctl start docker
usermod -aG docker ubuntu

mkdir -p /opt/velzard

cat << 'INNER_EOF' > /opt/velzard/velzard_engine.py
import os, sys, yaml

def generate_velzard_deployment():
    if not os.path.exists("velzion.yaml"): sys.exit(1)
    with open("velzion.yaml", 'r') as f: config = yaml.safe_load(f)
    
    byoc_mode = config.get("byoc_mode", False)
    routes = config.get("routes", {})
    
    if byoc_mode:
        # Enterprise Mode: Bring Your Own Compose
        if isinstance(byoc_mode, str):
            compose_file = byoc_mode
        else:
            possible_files = ["docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"]
            compose_file = next((f for f in possible_files if os.path.exists(f)), None)
        
        if not compose_file or not os.path.exists(compose_file): sys.exit(1)
        with open(compose_file, 'r') as f: compose = yaml.safe_load(f)
        
        if "networks" not in compose: compose["networks"] = {}
        compose["networks"]["velzion-network"] = {"driver": "bridge"}
        
        services = compose.get("services", {})
        for s_name, s_data in services.items():
            if "networks" not in s_data: s_data["networks"] = []
            if isinstance(s_data["networks"], list):
                if "velzion-network" not in s_data["networks"]: s_data["networks"].append("velzion-network")
            elif isinstance(s_data["networks"], dict):
                s_data["networks"]["velzion-network"] = {}
    else:
        # Standard Mode: Child's Play Contract
        services = config.get("services", {})
        database_config = config.get("database", {})
        has_database = bool(database_config)
        
        compose = {"version": "3.8", "services": {}, "networks": {"velzion-network": {"driver": "bridge"}}, "volumes": {}}
        
        db_env_injection = ""
        if has_database:
            db_engine = database_config.get("engine", "postgres:16-alpine").lower()
            db_service = {"image": db_engine, "networks": ["velzion-network"], "restart": "always"}
            
            if "postgres" in db_engine:
                db_service.update({"environment": ["POSTGRES_USER=velzion_prod", "POSTGRES_PASSWORD=secure_prod_123", "POSTGRES_DB=velzion_db"], "volumes": ["db_data:/var/lib/postgresql/data"]})
                db_env_injection = "DATABASE_URL=postgres://velzion_prod:secure_prod_123@database:5432/velzion_db"
            elif "mongo" in db_engine:
                db_service.update({"environment": ["MONGO_INITDB_ROOT_USERNAME=velzion_admin", "MONGO_INITDB_ROOT_PASSWORD=secure_prod_123", "MONGO_INITDB_DATABASE=velzion_db"], "volumes": ["db_data:/data/db"]})
                db_env_injection = "MONGO_URI=mongodb://velzion_admin:secure_prod_123@database:27017/velzion_db?authSource=admin"
            elif "mysql" in db_engine:
                db_service.update({"environment": ["MYSQL_ROOT_PASSWORD=secure_prod_123", "MYSQL_DATABASE=velzion_db"], "volumes": ["db_data:/var/lib/mysql"]})
                db_env_injection = "MYSQL_URL=mysql://root:secure_prod_123@database:3306/velzion_db"
            elif "redis" in db_engine:
                db_env_injection = "REDIS_URL=redis://database:6379"

            compose["services"]["database"] = db_service
            compose["volumes"]["db_data"] = {}
        
        for s_name, s_data in services.items():
            b_path = s_data.get("path", ".")
            if not os.path.exists(os.path.join(b_path, "Dockerfile")): sys.exit(1)
            
            s_entry = {
                "build": {"context": b_path}, 
                "networks": ["velzion-network"], 
                "expose": [str(s_data.get("port", 80))], 
                "environment": s_data.get("env", []),
                "restart": s_data.get("restart", "unless-stopped")
            }
            
            reserved_keys = ["path", "port", "env", "needs", "restart"]
            for key, value in s_data.items():
                if key not in reserved_keys:
                    s_entry[key] = value
            
            if "needs" in s_data and "database" in s_data["needs"] and has_database:
                if "depends_on" not in s_entry: s_entry["depends_on"] = []
                if isinstance(s_entry["depends_on"], list): s_entry["depends_on"].append("database")
                elif isinstance(s_entry["depends_on"], dict): s_entry["depends_on"]["database"] = {"condition": "service_started"}
                if db_env_injection: s_entry["environment"].append(db_env_injection)
                    
            compose["services"][s_name] = s_entry
    if not byoc_mode:
        compose["services"]["gateway"] = {
            "image": "nginx:alpine", 
            "ports": ["80:80"], 
            "volumes": ["./nginx.conf:/etc/nginx/conf.d/default.conf:ro"], 
            "networks": ["velzion-network"], 
            "depends_on": list(services.keys())
        }

    compose["services"]["telemetry"] = {
        "image": "otel/opentelemetry-collector-contrib:latest",
        "container_name": "velzion-telemetry",
        "user": "0:0", 
        "command": ["--config=/etc/otelcol/config.yaml"],
        "volumes": [
            "./otel-config.yaml:/etc/otelcol/config.yaml:ro",
            "/var/run/docker.sock:/var/run/docker.sock:ro"
        ],
        "networks": ["velzion-network"],
        "restart": "always"
    }
    
    nginx_conf = ["server {", "    listen 80;", ""]
    for r_path, t_service in routes.items():
        t_port = services.get(t_service, {}).get("port", 80)
        nginx_conf.extend([
            f"    location {r_path} {{", 
            f"        proxy_pass http://{t_service}:{t_port};", 
            "        proxy_set_header Host $host;", 
            "        proxy_set_header X-Real-IP $remote_addr;", 
            "        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;",
            "    }\n"
        ])
    nginx_conf.append("}")
    
    with open("docker-compose.yml", "w") as f: yaml.dump(compose, f, default_flow_style=False, sort_keys=False)
    with open("nginx.conf", "w") as f: f.write("\n".join(nginx_conf))

if __name__ == "__main__": generate_velzard_deployment()
INNER_EOF

git clone ${var.repo_url} /app
cd /app
git checkout ${var.branch}

cat << 'OTEL_EOF' > /app/otel-config.yaml
receivers:
  docker_stats:
    endpoint: unix:///var/run/docker.sock
    collection_interval: 10s
    timeout: 5s

processors:
  batch:
    timeout: 10s

exporters:
  otlphttp:
    metrics_endpoint: "${var.backend_url}/api/velzard/deployments/${var.deployment_id}/telemetry/"
    encoding: json
    compression: none
    headers:
      "x-velzion-secret": "L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA"
  debug:
    verbosity: basic

service:
  pipelines:
    metrics:
      receivers: [docker_stats]
      processors: [batch]
      exporters: [otlphttp, debug]
OTEL_EOF

curl -X PATCH ${var.backend_url}/api/velzard/deployments/${var.deployment_id}/webhook_update/ \
  -H "x-velzion-secret: L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA" \
  -H "Content-Type: application/json" \
  -d '{"status": "COMPILING"}'

python3 /opt/velzard/velzard_engine.py

if [ $? -eq 0 ]; then
  docker compose up -d --build
  
  curl -X PATCH ${var.backend_url}/api/velzard/deployments/${var.deployment_id}/webhook_update/ \
    -H "x-velzion-secret: L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA" \
    -H "Content-Type: application/json" \
    -d '{"status": "RUNNING"}'
fi
EOF

  tags = {
    Name = "velzard-production-node"
  }
}

resource "aws_eip" "velzard_prod_ip" {
  instance = aws_instance.velzard_prod_server.id
  domain   = "vpc"
}

output "instance_id" {
  value       = aws_instance.velzard_prod_server.id
  description = "The persistent ID of the production server"
}

output "public_ip" {
  value       = aws_eip.velzard_prod_ip.public_ip
  description = "The unchangeable Production Elastic Public IP Gateway"
}