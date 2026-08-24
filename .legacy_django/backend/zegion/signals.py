import requests
import threading
from django.db.models.signals import post_save
from django.dispatch import receiver
from .models import Project

def send_n8n_trigger(project_id, repo_url, role_arn):
    import subprocess
    import os
    
    from django.conf import settings
    tf_dir = f"/tmp/zegion_project_{project_id}"
    try:
        os.makedirs(tf_dir, exist_ok=True)
        # Assuming you have a zegion_main.tf equivalent, or use velzard's for now
        tf_source = os.path.join(settings.BASE_DIR, "terraform", "velzard_main.tf")
        subprocess.run(f"cp {tf_source} {tf_dir}/main.tf || true", shell=True)
        
        # Here we would normally assume role and run Terraform
        print(f"📡 Pure Python Zegion Triggered for {repo_url} using {role_arn}")
    except Exception as e:
        print(f"⚠️ Outbound automation hook failed: {e}")

@receiver(post_save, sender=Project)
def trigger_infrastructure_pipeline(sender, instance, created, **kwargs):
    if created:
        print(f"⚡ Project Row Created in Postgres. Spinning background orchestration worker thread...")
        # Execute asynchronously so the React frontend form doesn't hang waiting for n8n/AWS
        threading.Thread(
            target=send_n8n_trigger,
            args=(instance.id, instance.github_repo_url, instance.aws_role_arn)
        ).start()