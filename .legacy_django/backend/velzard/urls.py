from django.urls import path, include
from rest_framework.routers import DefaultRouter
from .views import ProductionDeploymentViewSet

router = DefaultRouter()
router.register(r'deployments', ProductionDeploymentViewSet, basename='production-deployment')

urlpatterns = [
    path('', include(router.urls)),
]