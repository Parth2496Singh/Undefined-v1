CREATE TABLE IF NOT EXISTS deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id),
    github_repo_url VARCHAR(255) NOT NULL,
    branch VARCHAR(255) DEFAULT 'main',
    instance_type VARCHAR(50) DEFAULT 't3.small',
    volume_size INT DEFAULT 30,
    status VARCHAR(50) DEFAULT 'PROVISIONING',
    error_logs TEXT,
    aws_instance_id VARCHAR(255),
    elastic_ip VARCHAR(255),
    telemetry_history JSONB DEFAULT '[]'::jsonb,
    config_snapshot JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
