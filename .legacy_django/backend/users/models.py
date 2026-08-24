from django.contrib.auth.models import AbstractUser
from django.db import models

class UserProfile(AbstractUser):
    github_id = models.CharField(max_length=255, unique=True, null=True, blank=True)
    github_access_token = models.CharField(max_length=255, null=True, blank=True)
    avatar_url = models.URLField(null=True, blank=True)
    
    #NEW: The secure vault for this specific user's authorized workspaces
    allowed_repos = models.JSONField(default=list, blank=True)
    
    # AWS IAM integration
    aws_iam_role_arn = models.CharField(max_length=255, null=True, blank=True)

    def __str__(self):
        return self.username