import uuid
from django.db import models
from django.conf import settings

class ProductionDeployment(models.Model):
    STATUS_CHOICES = [
        ('OFFLINE', 'Offline'),
        ('VERIFYING', 'Verifying Contract'),
        ('PROVISIONING', 'Provisioning EC2'),
        ('COMPILING', 'Compiling Code'),
        ('RUNNING', 'Running'),
        ('DESTROYING', 'Terminating Cluster'),
        ('FAILED', 'Failed'),
    ]

    id = models.UUIDField(primary_key=True, default=uuid.uuid4, editable=False)
    user = models.ForeignKey(settings.AUTH_USER_MODEL, on_delete=models.CASCADE, related_name='production_deployments')
    
    # Target Information
    github_repo_url = models.CharField(max_length=255)
    branch = models.CharField(max_length=50, default='main')
    
    # Infrastructure Sizing
    instance_type = models.CharField(max_length=50, default='t3.small')
    volume_size = models.IntegerField(default=30)
    
    # Infrastructure State
    aws_instance_id = models.CharField(max_length=100, blank=True, null=True)
    elastic_ip = models.GenericIPAddressField(blank=True, null=True)
    status = models.CharField(max_length=20, choices=STATUS_CHOICES, default='OFFLINE')
    
    # Telemetry Rolling Cache for the UI
    telemetry_history = models.JSONField(default=list, blank=True, help_text="Rolling 15-tick history of CPU/RAM")
    container_status = models.JSONField(default=list, blank=True, help_text="Parsed active container list")
    
    config_snapshot = models.JSONField(blank=True, null=True, help_text="The parsed velzion.yaml matrix")
    
    created_at = models.DateTimeField(auto_now_add=True)
    updated_at = models.DateTimeField(auto_now=True)
    ascended_at = models.DateTimeField(blank=True, null=True, help_text="Timestamp when infrastructure went live")

    def __str__(self):
        return f"Production: {self.github_repo_url} - {self.status}"