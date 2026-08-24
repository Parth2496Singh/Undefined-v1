import os
import requests
from django.contrib.auth import login
from rest_framework.views import APIView
from rest_framework.response import Response
# NEW: Import AllowAny
from rest_framework.permissions import AllowAny 
from .models import UserProfile

class GitHubLoginURLView(APIView):
    permission_classes = [AllowAny] # Good practice to leave this open too
    def get(self, request):
        client_id = os.environ.get('GITHUB_CLIENT_ID')
        redirect_uri = f"{os.environ.get('FRONTEND_URL')}/auth/callback"
        
        url = f"https://github.com/login/oauth/authorize?client_id={client_id}&redirect_uri={redirect_uri}&scope=read:user,user:email,repo"
        return Response({"login_url": url})

class GitHubCallbackView(APIView):
    # Drop the CSRF and Auth shields for this specific endpoint
    authentication_classes = [] 
    permission_classes = [AllowAny] 

    def post(self, request):
        code = request.data.get('code')
        
        # 1. Exchange code for access token
        token_response = requests.post(
            'https://github.com/login/oauth/access_token',
            headers={'Accept': 'application/json'},
            data={
                'client_id': os.environ.get('GITHUB_CLIENT_ID'),
                'client_secret': os.environ.get('GITHUB_CLIENT_SECRET'),
                'code': code,
            }
        )
        access_token = token_response.json().get('access_token')

        if not access_token:
            return Response({"error": "Failed to get access token"}, status=400)

        # 2. Get GitHub profile
        user_response = requests.get(
            'https://api.github.com/user',
            headers={'Authorization': f'Bearer {access_token}'}
        )
        github_user = user_response.json()

        # 3. Create or Log in user
        user, created = UserProfile.objects.get_or_create(
            github_id=github_user['id'],
            defaults={
                'username': github_user['login'],
                'avatar_url': github_user.get('avatar_url', ''),
                'github_access_token': access_token
            }
        )
        
        if not created:
            user.github_access_token = access_token
            user.save()

        from django.middleware.csrf import get_token
        login(request, user)
        get_token(request) # Force Django to set the csrftoken cookie

        # 🔥 Fetch all workspaces (repos) this user has access to
        repos_list = []
        try:
            repos_response = requests.get(
                'https://api.github.com/user/repos?affiliation=owner,collaborator,organization_member&per_page=100',
                headers={'Authorization': f'Bearer {access_token}'},
                timeout=5
            )
            if repos_response.status_code == 200:
                repos_data = repos_response.json()
                repos_list = [repo['full_name'] for repo in repos_data]
                
                # 🛑 THE FIREWALL UPDATE: Save their clearance list directly into the database
                user.allowed_repos = repos_list
                user.save()
                
        except Exception as e:
            print(f"Warning: Failed to fetch repositories during login: {e}")
        # 4. Return user profile AND the workspace list to React
        return Response({
            "message": "Login successful",
            "user": {
                "username": user.username,
                "avatar": user.avatar_url
            },
            "repos": repos_list
        })

from django.contrib.auth import logout
class LogoutView(APIView):
    def post(self, request):
        logout(request)
        response = Response({"message": "Logged out successfully"})
        response.delete_cookie('csrftoken')
        response.delete_cookie('sessionid')
        return response

from rest_framework.permissions import IsAuthenticated
class BindIAMRoleView(APIView):
    permission_classes = [IsAuthenticated]
    
    def post(self, request):
        role_arn = request.data.get('arn')
        if not role_arn:
            return Response({"error": "ARN is required"}, status=400)
            
        request.user.aws_iam_role_arn = role_arn
        request.user.save()
        return Response({"message": "IAM Role bound to workspace successfully."})