from rest_framework import serializers
from .models import ProductionDeployment

class ProductionDeploymentSerializer(serializers.ModelSerializer):
    class Meta:
        model = ProductionDeployment
        fields = '__all__'
        read_only_fields = ['id', 'user', 'status', 'aws_instance_id', 'elastic_ip', 'config_snapshot', 'created_at', 'updated_at']