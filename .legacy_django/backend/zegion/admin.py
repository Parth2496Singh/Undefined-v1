from django.contrib import admin
from .models import Project, EphemeralEnvironment

admin.site.register(Project)
admin.site.register(EphemeralEnvironment)
