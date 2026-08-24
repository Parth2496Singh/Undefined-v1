import os
import requests
from django.utils import timezone
from django.conf import settings
from rest_framework import viewsets, status
from rest_framework.decorators import api_view, permission_classes, action
from rest_framework.permissions import AllowAny, IsAuthenticated
from rest_framework.response import Response
from rest_framework.views import APIView

from .models import Project, EphemeralEnvironment
from .serializers import ProjectSerializer, EphemeralEnvironmentSerializer

# --- GITHUB API UTILITY HELPER ---
def post_to_github_timeline(repo_full_name, pr_number, message_text):
    """
    Direct utility to allow Django to post error logs and alerts back to the GitHub PR timeline
    bypassing n8n when handling gatekeeper failures.
    """
    token = os.environ.get('GITHUB_PAT', 'YOUR_GITHUB_PAT_HERE') # Pulls from env or fallback
    url = f"https://api.github.com/repos/{repo_full_name}/issues/{pr_number}/comments"
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/vnd.github.v3+json"
    }
    try:
        requests.post(url, json={"body": message_text}, headers=headers, timeout=5)
    except Exception as e:
        print(f"Failed to post administrative message to GitHub: {str(e)}")


# --- REACT DASHBOARD ENDPOINTS ---
class ProjectViewSet(viewsets.ModelViewSet):
    queryset = Project.objects.all()
    serializer_class = ProjectSerializer

class EphemeralEnvironmentViewSet(viewsets.ModelViewSet):
    serializer_class = EphemeralEnvironmentSerializer

    # 🔥 SECURITY LOCK: Only logged-in users can hit this endpoint
    permission_classes = [IsAuthenticated]

    def get_queryset(self):
        user = self.request.user
        workspace_query = self.request.query_params.get('repo')
        
        # 1. If no workspace is requested, or user is anonymous, reject immediately
        if not workspace_query or not user.is_authenticated:
            return EphemeralEnvironment.objects.none()
            
        # 2. THE FIREWALL: Check if the requested workspace is in their database clearance list
        # (We use .lower() to ensure capitalization differences don't block legitimate access)
        is_authorized = any(
            workspace_query.lower() == authorized_repo.lower() 
            for authorized_repo in user.allowed_repos
        )
        
        # 3. If they try to request a repo they don't own, return an empty list
        if not is_authorized:
            return EphemeralEnvironment.objects.none()

        # 4. If they pass the firewall, run the fuzzy search safely!
        return EphemeralEnvironment.objects.filter(
            project__github_repo_url__icontains=workspace_query
        ).order_by('-created_at')


    # 🔥 FRONTEND UI OVERRIDE TERMINATION HOOK 🔥 (Preserved exactly as you had it)
    @action(detail=True, methods=['post'])
    def terminate(self, request, pk=None):
        environment = self.get_object()
        
        if environment.status == 'DESTROYED':
            return Response({"error": "Environment is already destroyed."}, status=status.HTTP_400_BAD_REQUEST)
            
        user_identity = request.data.get('user', 'Dashboard User')
        
        environment.status = 'DESTROYED'
        environment.terminated_by = user_identity
        environment.terminated_at = timezone.now()
        environment.save()
        
        # Pure Python Spot Instance Teardown
        import threading
        def run_terraform_destroy(env_id):
            import subprocess
            import os
            try:
                tf_dir = f"/tmp/zegion_{env_id}"
                subprocess.run(["terraform", "destroy", "-auto-approve"], cwd=tf_dir)
                subprocess.run(f"rm -rf {tf_dir}", shell=True)
                print(f"Zegion Spot Instance {env_id} natively destroyed.")
            except Exception as e:
                print(f"Failed to destroy Spot Instance natively: {e}")

        threading.Thread(target=run_terraform_destroy, args=(str(environment.id),)).start()
            
        return Response({
            "message": f"Permanent infrastructure teardown sequence initialized by {user_identity}.",
            "environment": EphemeralEnvironmentSerializer(environment).data
        }, status=status.HTTP_200_OK)


# --- N8N DYNAMIC LOOKUP ENDPOINT ---
@api_view(['GET'])
@permission_classes([AllowAny])
def lookup_tenant_arn(request):
    repo_url = request.GET.get('repo_url')
    if not repo_url:
        return Response({'error': 'Missing repo_url parameter'}, status=400)
    
    project = Project.objects.filter(github_repo_url=repo_url).first()
    if project:
        return Response({
            'client_role_arn': project.aws_role_arn,
            'status': 'FOUND'
        }, status=200)
    else:
        return Response({'error': 'No integration found for this repository'}, status=404)


# --- N8N / GITHUB STATE SYNCHRONIZATION WEBHOOK ---
@api_view(['POST'])
@permission_classes([AllowAny])
def github_webhook(request):
    received_secret = request.headers.get('x-velzion-secret')
    actual_secret = "L0JFLBRiyyWiCatJeju2IHXOm-yQUFuhSzjflv8q_a8SgeDP9SoKNeRmyE_xyCre5lZ0TpREAdxbK37q84IjfA"
    
    if received_secret != actual_secret:
        return Response({"error": "Unauthorized"}, status=403)
        
    data = request.data
    repo_url = data.get('repo_url')
    raw_pr_number = data.get('pr_number')
    action_status = data.get('status', 'PROVISIONING')
    instance_id = data.get('instance_id')
    dns_prefix = data.get('dns_prefix')

    if not repo_url or not raw_pr_number:
        return Response({"error": "Missing critical fields"}, status=status.HTTP_400_BAD_REQUEST)

    try:
        pr_number = int(raw_pr_number)
    except (ValueError, TypeError):
        return Response({"error": "pr_number must be an integer."}, status=status.HTTP_400_BAD_REQUEST)

    project, _ = Project.objects.get_or_create(
        github_repo_url=repo_url,
        defaults={
            'aws_role_arn': 'arn:aws:iam::815090125753:role/VelzionExecutionRole',
            'vpc_id': 'vpc-default-mvp'
        }
    )

    update_data = {'status': action_status}
    if instance_id: update_data['instance_id'] = instance_id
    if dns_prefix: update_data['dns_prefix'] = dns_prefix

    # If syncing back an automated close from GitHub webhook, log it cleanly
    if action_status == 'DESTROYED':
        update_data['terminated_at'] = timezone.now()
        update_data['terminated_by'] = 'GitHub Webhook (PR Closed)'

    environment, _ = EphemeralEnvironment.objects.update_or_create(
        project=project,
        pr_number=pr_number,
        defaults=update_data
    )
    if action_status == 'BUILT':
        n8n_base = os.environ.get('N8N_INTERNAL_URL', 'http://n8n:5678')
        try:
            requests.post(f"{n8n_base}/webhook/zegion-auto-sleep", json={
                "repo_url": repo_url,
                "pr_number": pr_number,
                "instance_id": environment.instance_id
            }, timeout=5)
        except Exception as e:
            # LOUD ERROR LOGGING
            print(f"❌ CRITICAL: Failed to trigger n8n auto-sleep: {str(e)}")

    return Response({
        "message": "State machine synchronized successfully.",
        "environment": EphemeralEnvironmentSerializer(environment).data
    }, status=status.HTTP_200_OK)


# --- ADVANCED CHATOPS GATEKEEPER VIEW ---
class GitHubCommentWebhookView(APIView):
    def post(self, request, *args, **kwargs):
        payload = request.data
        
        if payload.get('action') != 'created':
            return Response({"message": "Ignored non-creation event"}, status=status.HTTP_200_OK)
            
        comment_body = payload.get('comment', {}).get('body', '').strip().lower()
        
        # 1. Route validation mapping
        if comment_body not in ['/zegion wake', '/zegion sleep']:
            return Response({"message": "Not an understood Zegion command"}, status=status.HTTP_200_OK)
            
        repo_url = payload.get('repository', {}).get('clone_url')
        repo_full_name = payload.get('repository', {}).get('full_name')
        pr_number = payload.get('issue', {}).get('number')
        
        if not pr_number or not repo_url:
            return Response({"error": "Malformed GitHub payload"}, status=status.HTTP_400_BAD_REQUEST)

        # 2. State Machine Query (The Guardrail check)
        env = EphemeralEnvironment.objects.filter(project__github_repo_url=repo_url, pr_number=pr_number).first()
        
        if not env:
            return Response({"message": "No active environment mapping exists for this PR record."}, status=status.HTTP_200_OK)

        # 🛑 EDGE CASE CRITICAL ERROR HANDLING: Already Destroyed 🛑
        if env.status == 'DESTROYED':
            timestamp_str = env.terminated_at.strftime('%Y-%m-%d %H:%M:%S UTC') if env.terminated_at else "Unknown Time"
            executor = env.terminated_by or "Administrative System"
            
            error_markdown = (
                f"### ❌ Zegion Control Plane Error\n"
                f"The command `{comment_body}` could not be executed.\n\n"
                f"* **Reason:** The infrastructure for PR #{pr_number} has been permanently de-provisioned.\n"
                f"* **Terminated By:** `{executor}`\n"
                f"* **Termination Time:** `{timestamp_str}`\n\n"
                f"⚠️ _Once an environment is terminated via the Dashboard or Merged, it cannot be awoken. You must open a new PR to re-provision container builds._"
            )
            post_to_github_timeline(repo_full_name, pr_number, error_markdown)
            return Response({"message": "Command blocked: Environment is permanently destroyed."}, status=status.HTTP_200_OK)

        # 🚦 WAKE COMMAND GUARDRAILS
        if comment_body == '/zegion wake':
            if env.status == 'RUNNING':
                post_to_github_timeline(repo_full_name, pr_number, "⚠️ **Zegion Engine Notice:** This environment is already active and routing traffic live.")
                return Response({"message": "Command blocked: Already running"}, status=status.HTTP_200_OK)
            
            # Transition state machine to prevent duplicate rapid clicks
            env.status = 'WAKING'
            env.save()
            
            # Fire to n8n Wake webhook endpoint
            n8n_base = os.environ.get('N8N_INTERNAL_URL', 'http://n8n:5678')
            requests.post(f"{n8n_base}/webhook/zegion-chatops-wake", json={
                "repo_url": repo_url,
                "pr_number": pr_number,
                "instance_id": env.instance_id
            }, timeout=5)

        # 💤 SLEEP COMMAND GUARDRAILS
        elif comment_body == '/zegion sleep':
            if env.status == 'SLEEPING':
                post_to_github_timeline(repo_full_name, pr_number, "⚠️ **Zegion Engine Notice:** This environment is already sleeping. Compute draw is already at $0.00/hr.")
                return Response({"message": "Command blocked: Already sleeping"}, status=status.HTTP_200_OK)
            
            # Fire to n8n Sleep webhook endpoint
            n8n_base = os.environ.get('N8N_INTERNAL_URL', 'http://n8n:5678')
            requests.post(f"{n8n_base}/webhook/zegion-chatops-sleep", json={
                "repo_url": repo_url,
                "pr_number": pr_number,
                "instance_id": env.instance_id
            }, timeout=5)

        return Response({"message": f"{comment_body} workflow routed to orchestration plane."}, status=status.HTTP_200_OK)