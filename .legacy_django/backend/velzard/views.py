import os
import uuid
import requests
import datetime
from django.utils import timezone
from rest_framework import viewsets, status
from rest_framework.decorators import action
from rest_framework.response import Response
from rest_framework.permissions import IsAuthenticated, AllowAny 
import yaml
import boto3

from .models import ProductionDeployment
from .serializers import ProductionDeploymentSerializer

def get_temp_aws_credentials(user):
    if not user.aws_iam_role_arn:
        return {}
    try:
        sts_client = boto3.client('sts')
        assumed_role = sts_client.assume_role(
            RoleArn=user.aws_iam_role_arn,
            RoleSessionName=f"VelzionSession-{user.username}"
        )
        return {
            "aws_access_key_id": assumed_role['Credentials']['AccessKeyId'],
            "aws_secret_access_key": assumed_role['Credentials']['SecretAccessKey'],
            "aws_session_token": assumed_role['Credentials']['SessionToken']
        }
    except Exception as e:
        print(f"Failed to assume role: {e}")
        return {}

class ProductionDeploymentViewSet(viewsets.ModelViewSet):
    serializer_class = ProductionDeploymentSerializer
    permission_classes = [IsAuthenticated]

    def get_queryset(self):
        if self.request.user.is_anonymous:
            return ProductionDeployment.objects.all()
        return ProductionDeployment.objects.filter(user=self.request.user)

    def perform_create(self, serializer):
        serializer.save(user=self.request.user)

    @action(detail=True, methods=['post'])
    def verify_contract(self, request, pk=None):
        deployment = self.get_object()
        user_token = request.user.github_access_token

        if not user_token:
            return Response({"error": "GitHub access token missing. Please re-authenticate."}, status=401)

        repo_name = deployment.github_repo_url.replace("https://github.com/", "")
        github_url = f"https://api.github.com/repos/{repo_name}/contents/velzion.yaml"
        headers = {
            "Authorization": f"Bearer {user_token}",
            "Accept": "application/vnd.github.v3+json"
        }

        response = requests.get(github_url, headers=headers)

        if response.status_code == 200:
            import base64
            content_b64 = response.json().get('content', '')
            try:
                yaml_content = base64.b64decode(content_b64).decode('utf-8')
                parsed_yaml = yaml.safe_load(yaml_content)
                deployment.config_snapshot = parsed_yaml
                deployment.save()
            except Exception as e:
                print("Failed to parse YAML snapshot:", e)

            return Response({"message": "Contract Verified! Ready for deployment.", "verified": True}, status=200)
        else:
            return Response({"error": "velzion.yaml not found in the repository root.", "verified": False}, status=404)

    @action(detail=True, methods=['post'])
    def trigger_deploy(self, request, pk=None):
        deployment = self.get_object()
        
        if not deployment.config_snapshot:
             return Response({"error": "Cannot deploy. Contract not verified."}, status=400)

        deployment.status = 'PROVISIONING'
        deployment.save()

        # Pure Python Infrastructure Trigger
        import subprocess
        import threading
        
        def run_terraform_deploy(dep_id, repo, branch, i_type, vol_size, aws_creds):
            from django.conf import settings
            try:
                tf_dir = f"/tmp/velzard_{dep_id}"
                os.makedirs(tf_dir, exist_ok=True)
                # Copy the main TF file securely using BASE_DIR
                tf_source = os.path.join(settings.BASE_DIR, "terraform", "velzard_main.tf")
                subprocess.run(f"cp {tf_source} {tf_dir}/main.tf", shell=True)
                
                env = os.environ.copy()
                if aws_creds:
                    env['AWS_ACCESS_KEY_ID'] = aws_creds.get('aws_access_key_id', '')
                    env['AWS_SECRET_ACCESS_KEY'] = aws_creds.get('aws_secret_access_key', '')
                    env['AWS_SESSION_TOKEN'] = aws_creds.get('aws_session_token', '')

                # Initialize & Apply
                subprocess.run(["terraform", "init"], cwd=tf_dir, env=env)
                subprocess.run([
                    "terraform", "apply", "-auto-approve",
                    f"-var=repo_url={repo}",
                    f"-var=branch={branch}",
                    f"-var=deployment_id={dep_id}",
                    f"-var=instance_type={i_type}",
                    f"-var=volume_size={vol_size}",
                    f"-var=backend_url=http://localhost:8000"
                ], cwd=tf_dir, env=env)
                
                deployment.status = 'RUNNING'
                deployment.save()
            except Exception as e:
                deployment.status = 'FAILED'
                deployment.save()
                print(f"Terraform Background Error: {e}")

        creds = get_temp_aws_credentials(request.user)
        threading.Thread(target=run_terraform_deploy, args=(str(deployment.id), deployment.github_repo_url, deployment.branch, deployment.instance_type, deployment.volume_size, creds)).start()

        return Response({"message": "Deployment orchestrated via pure Python. Booting production servers."}, status=200)

    @action(detail=True, methods=['post'])
    def destroy_cluster(self, request, pk=None):
        deployment = self.get_object()
        
        if deployment.status in ["DESTROYING", "DESTROYED"]:
            return Response({"error": "Deployment is already terminating or destroyed."}, status=400)

        deployment.status = 'DESTROYING'
        deployment.save()

        # Pure Python Infrastructure Teardown
        import subprocess
        import threading
        
        def run_terraform_destroy(dep_id, repo, branch, i_type, vol_size, aws_creds):
            try:
                tf_dir = f"/tmp/velzard_{dep_id}"
                env = os.environ.copy()
                if aws_creds:
                    env['AWS_ACCESS_KEY_ID'] = aws_creds.get('aws_access_key_id', '')
                    env['AWS_SECRET_ACCESS_KEY'] = aws_creds.get('aws_secret_access_key', '')
                    env['AWS_SESSION_TOKEN'] = aws_creds.get('aws_session_token', '')

                subprocess.run([
                    "terraform", "destroy", "-auto-approve",
                    f"-var=repo_url={repo}",
                    f"-var=branch={branch}",
                    f"-var=deployment_id={dep_id}",
                    f"-var=instance_type={i_type}",
                    f"-var=volume_size={vol_size}",
                    f"-var=backend_url=http://localhost:8000"
                ], cwd=tf_dir, env=env)
                
                subprocess.run(f"rm -rf {tf_dir}", shell=True)
                deployment.status = 'DESTROYED'
                deployment.save()
            except Exception as e:
                print(f"Terraform Background Teardown Error: {e}")

        creds = get_temp_aws_credentials(request.user)
        threading.Thread(target=run_terraform_destroy, args=(str(deployment.id), deployment.github_repo_url, deployment.branch, deployment.instance_type, deployment.volume_size, creds)).start()

        return Response({"message": "Teardown initiated via pure Python. Cluster is being destroyed."}, status=200)

    @action(detail=True, methods=['patch'], permission_classes=[AllowAny])
    def webhook_update(self, request, pk=None):
        secret = request.headers.get('x-velzion-secret')
        expected_secret = os.environ.get('N8N_WEBHOOK_SECRET', 'L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA')
        
        if secret != expected_secret:
            return Response({"error": "Unauthorized webhook signature."}, status=403)

        deployment = self.get_object()
        new_status = request.data.get('status')
        
        if new_status in ['DELETED', 'DESTROYED']:
            deployment.delete()
            return Response({"message": "Deployment record wiped from matrix."}, status=200)

        aws_instance_id = request.data.get('aws_instance_id')
        elastic_ip = request.data.get('elastic_ip')

        if not new_status and aws_instance_id and elastic_ip:
            new_status = 'RUNNING'

        if new_status == 'RUNNING' and deployment.status != 'RUNNING':
            deployment.ascended_at = timezone.now()

        deployment.status = new_status or deployment.status
        deployment.aws_instance_id = request.data.get('aws_instance_id', deployment.aws_instance_id)
        deployment.elastic_ip = request.data.get('elastic_ip', deployment.elastic_ip)
        deployment.save()

        return Response({"message": "Deployment state synced with AWS."}, status=200)

# The OTLP Telemetry Ingestion Engine
    @action(detail=True, methods=['post'], permission_classes=[AllowAny])
    def telemetry(self, request, pk=None):
        secret = request.headers.get('x-velzion-secret')
        if secret != os.environ.get('N8N_WEBHOOK_SECRET', 'L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA'):
            return Response({"error": "Unauthorized telemetry source."}, status=403)

        deployment = self.get_object()
        raw_data = request.data
        
        cpu_ns_total = 0.0 # 🚀 NEW: Tracking raw CPU time
        ram_bytes_total = 0.0
        system_ram_limit_bytes = 0.0 
        active_containers = set()

        try:
            resource_metrics = raw_data.get("resourceMetrics", [])
            for rm in resource_metrics:
                attributes = rm.get("resource", {}).get("attributes", [])
                for attr in attributes:
                    if attr.get("key") == "container.name":
                        active_containers.add(attr.get("value", {}).get("stringValue", "unknown"))

                scope_metrics = rm.get("scopeMetrics", [])
                for sm in scope_metrics:
                    metrics = sm.get("metrics", [])
                    for metric in metrics:
                        name = metric.get("name", "")
                        
                        datapoints = []
                        if "gauge" in metric:
                            datapoints = metric["gauge"].get("dataPoints", [])
                        elif "sum" in metric:
                            datapoints = metric["sum"].get("dataPoints", [])

                        if not datapoints: continue
                        
                        val = float(datapoints[0].get("asDouble", datapoints[0].get("asInt", 0)))

                        if name == "container.cpu.usage.total":
                            # 🚀 FIXED: Accumulate the true raw nanoseconds
                            cpu_ns_total += val
                            
                        if name == "container.memory.usage.total":
                            ram_bytes_total += val
                            
                        if name == "container.memory.usage.limit":
                            if val > system_ram_limit_bytes:
                                system_ram_limit_bytes = val

        except Exception as e:
            print(f"OTLP Parsing Warning: {e}")

        # --- RAM MATH (Dynamic & Accurate) ---
        if system_ram_limit_bytes > 0:
            ram_val = min((ram_bytes_total / system_ram_limit_bytes) * 100, 99.9)
        else:
            ram_val = min((ram_bytes_total / (2048.0 * 1024 * 1024)) * 100, 99.9)

        # --- CPU MATH (The Real Odometer Delta) ---
        last_cpu_ns = cpu_ns_total
        
        # 1. Reach into the database and grab the nanoseconds from 10 seconds ago
        if deployment.telemetry_history:
            last_cpu_ns = deployment.telemetry_history[-1].get("raw_cpu_ns", cpu_ns_total)
            
        # 2. Calculate the difference
        delta_ns = cpu_ns_total - last_cpu_ns
        
        # 3. OTel sends data every 10 seconds (10,000,000,000 nanoseconds).
        # A t3.small has 2 vCPUs, meaning the max possible compute time in a 10s window is 20,000,000,000 ns.
        # We divide the delta by the total capacity of the server to get 0-100%.
        if delta_ns > 0:
            cpu_val = min((delta_ns / 20_000_000_000.0) * 100, 99.9)
        else:
            cpu_val = 0.5 # Idle baseline

        container_list = []
        for c in active_containers:
            if "velzion-telemetry" not in c:
                container_list.append({
                    "ID": str(uuid.uuid4())[:8],
                    "Names": c.replace("/", ""), 
                    "State": "Up",
                    "Ports": "Internal/Gateway Network"
                })

        current_time = datetime.datetime.now().strftime("%H:%M:%S")
        new_metric = {
            "time": current_time,
            "cpu": round(cpu_val, 2), 
            "ram": round(ram_val, 2),
            "raw_cpu_ns": cpu_ns_total # 🚀 SECRET: Save current nanoseconds for the next math cycle!
        }
        
        current_history = deployment.telemetry_history
        current_history.append(new_metric)
        
        deployment.telemetry_history = current_history[-15:]
        
        if container_list:
            deployment.container_status = container_list
            
        deployment.save()

        return Response({"status": "OTLP Ingestion Successful"}, status=200)