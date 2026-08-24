import uuid
from django.db import models

class Project(models.Model):
    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    github_repo_url = models.URLField(max_length=255)
    aws_role_arn = models.CharField(max_length=255)
    vpc_id = models.CharField(max_length=50)
    created_at = models.DateTimeField(auto_now_add=True)

    def __str__(self):
        return f"Project: {self.github_repo_url}"

class EphemeralEnvironment(models.Model):
    STATUS_CHOICES = [
        ('PROVISIONING', 'Provisioning'),
        ('RUNNING', 'Running'),
        ('BUILT', 'Built'),
        ('SLEEPING', 'Sleeping'),
        ('WAKING', 'Waking'),       # 🔥 Added for native start transition tracking
        ('DESTROYED', 'Destroyed'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    project = models.ForeignKey(Project, on_delete=models.CASCADE, related_name='environments')
    pr_number = models.IntegerField()
    status = models.CharField(max_length=20, choices=STATUS_CHOICES, default='PROVISIONING')
    dns_prefix = models.CharField(max_length=50, blank=True, null=True)
    instance_id = models.CharField(max_length=50, blank=True, null=True)
    ttl_expires_at = models.DateTimeField(blank=True, null=True)
    
    # 🔥 FINOPS & AUDIT TRAIL TRACKING FIELD UPGRADES 🔥
    terminated_at = models.DateTimeField(blank=True, null=True)
    terminated_by = models.CharField(max_length=100, blank=True, null=True)
    
    updated_at = models.DateTimeField(auto_now=True)
    created_at = models.DateTimeField(auto_now_add=True)

    def __str__(self):
        return f"PR #{self.pr_number} - {self.status}"