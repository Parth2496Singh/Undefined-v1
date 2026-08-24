from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import ProjectViewSet, EphemeralEnvironmentViewSet, github_webhook, lookup_tenant_arn, GitHubCommentWebhookView

# Auto-generate standard REST routes
router = DefaultRouter()
router.register(r'projects', ProjectViewSet)
router.register(r'environments', EphemeralEnvironmentViewSet, basename='environment')

urlpatterns = [
    # This includes all the router-generated URLs
    path('', include(router.urls)),
    # This maps our custom webhook endpoint
    path('webhooks/github/', github_webhook, name='github-webhook'),
    #the new ChatOps webhook
    path('webhooks/github/comments/', GitHubCommentWebhookView.as_view(), name='github-comments'),
    # NEW: Endpoint for n8n to query the ARN dynamically
    path('integrations/lookup/', lookup_tenant_arn, name='lookup-tenant-arn'),
]