CREATE TABLE IF NOT EXISTS ephemeral_environments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    github_repo_url VARCHAR(255) NOT NULL,
    pr_number INT NOT NULL,
    status VARCHAR(50) NOT NULL,
    dns_prefix VARCHAR(50),
    instance_id VARCHAR(100),
    error_logs TEXT,
    ttl_expires_at TIMESTAMP WITH TIME ZONE,
    terminated_at TIMESTAMP WITH TIME ZONE,
    terminated_by VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, github_repo_url, pr_number)
);
