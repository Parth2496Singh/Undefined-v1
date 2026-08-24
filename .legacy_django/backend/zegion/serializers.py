from rest_framework import serializers
from .models import Project, EphemeralEnvironment

class ProjectSerializer(serializers.ModelSerializer):
    class Meta:
        model = Project
        fields = '__all__'

class EphemeralEnvironmentSerializer(serializers.ModelSerializer):
    class Meta:
        model = EphemeralEnvironment
        fields = '__all__'