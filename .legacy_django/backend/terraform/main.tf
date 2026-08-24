variable "repo_url" {
  description = "The GitHub repository to clone and build"
  type        = string
}

variable "pr_number" {
  description = "The PR number being deployed"
  type        = string
}

# 100% KEYLESS PROVIDER
provider "aws" {
  region = "us-east-1"
  # Terraform will automatically pull credentials from ~/.aws/credentials (locally)
  # or the attached IAM Role (when running on EC2).
}

resource "aws_security_group" "zegion_sg" {
  name        = "zegion_preview_sg_${var.pr_number}"
  description = "Allow HTTP and SSH inbound for PR preview"
}

resource "aws_vpc_security_group_ingress_rule" "allow_http_ipv4" {
  security_group_id = aws_security_group.zegion_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 80
  ip_protocol       = "tcp"
  to_port           = 80
  description       = "Anyone can access"
}

resource "aws_vpc_security_group_ingress_rule" "allow_ssh_ipv4" {
  security_group_id = aws_security_group.zegion_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  from_port         = 22
  ip_protocol       = "tcp"
  to_port           = 22
  description       = "Emergency SSH access"
}

resource "aws_vpc_security_group_egress_rule" "allow_all_traffic_ipv4" {
  security_group_id = aws_security_group.zegion_sg.id
  cidr_ipv4         = "0.0.0.0/0"
  ip_protocol       = "-1" 
  description       = "EC2 can connect to the whole world to download code"
}

# SPOT INSTANCES!
resource "aws_spot_instance_request" "zegion_preview" {
  ami                    = "ami-04900bc2bfac97d75" # Ubuntu 24.04 LTS
  instance_type          = "t3.micro"
  spot_price             = "0.01"
  wait_for_fulfillment   = true
  vpc_security_group_ids = [aws_security_group.zegion_sg.id]

  # 🔥 THE OPTIMIZED ZEGION STATE-MACHINE UPGRADES 🔥
  spot_type                      = "persistent"
  instance_interruption_behavior = "stop"

  root_block_device {
    volume_size = 20
    volume_type = "gp3" 
  }

  user_data = <<-EOF
              #!/bin/bash 
              exec > >(tee /var/log/user-data.log|logger -t user-data -s 2>/dev/console) 2>&1
              export HOME=/root
              
              # 1. Clone the repo
              git clone ${var.repo_url} /app
              cd /app
                              
              # 2. Fetch the specific PR branch
              git fetch origin pull/${var.pr_number}/head:pr-preview
              git checkout pr-preview

              # 3. Build using pre-installed Pack (Images are already on disk!)
              pack build zegion-app --builder paketobuildpacks/builder-jammy-base
                              
              # 4. Run using pre-installed Docker
              docker run -d -p 80:8080 --name zegion-web --restart unless-stopped zegion-app

              # 5. Send Webhook
              curl -X POST http://54.86.145.100/api/webhooks/github/ \
                -H "x-velzion-secret: L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA" \
                -H "Content-Type: application/json" \
                -d '{"repo_url": "'"${var.repo_url}"'", "pr_number": "'"${var.pr_number}"'", "status": "BUILT"}'
              EOF
}

output "instance_id" {
  value = aws_spot_instance_request.zegion_preview.spot_instance_id
}

output "public_ip" {
  value = aws_spot_instance_request.zegion_preview.public_ip
}