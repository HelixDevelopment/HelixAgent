-- SuperAgent Database Initialization Script
-- This script sets up the initial database schema for testing

-- Create database if it doesn't exist
CREATE DATABASE IF NOT EXISTS superagent_db;

-- Connect to the database
\c superagent_db;

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(255) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    api_key VARCHAR(255) UNIQUE NOT NULL,
    role VARCHAR(50) DEFAULT 'user',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create user_sessions table
CREATE TABLE IF NOT EXISTS user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    session_token VARCHAR(255) UNIQUE NOT NULL,
    context JSONB DEFAULT '{}',
    memory_id UUID,
    status VARCHAR(50) DEFAULT 'active',
    request_count INTEGER DEFAULT 0,
    last_activity TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create llm_providers table
CREATE TABLE IF NOT EXISTS llm_providers (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) UNIQUE NOT NULL,
    type VARCHAR(100) NOT NULL,
    api_key VARCHAR(255),
    base_url VARCHAR(500),
    model VARCHAR(255),
    weight DECIMAL(5,2) DEFAULT 1.0,
    enabled BOOLEAN DEFAULT TRUE,
    config JSONB DEFAULT '{}',
    health_status VARCHAR(50) DEFAULT 'unknown',
    response_time BIGINT DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create llm_requests table
CREATE TABLE IF NOT EXISTS llm_requests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    prompt TEXT NOT NULL,
    messages JSONB NOT NULL DEFAULT '[]',
    model_params JSONB NOT NULL DEFAULT '{}',
    ensemble_config JSONB DEFAULT NULL,
    memory_enhanced BOOLEAN DEFAULT FALSE,
    memory JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    request_type VARCHAR(50) DEFAULT 'completion'
);

-- Create llm_responses table
CREATE TABLE IF NOT EXISTS llm_responses (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    request_id UUID REFERENCES llm_requests(id) ON DELETE CASCADE,
    provider_id UUID REFERENCES llm_providers(id) ON DELETE SET NULL,
    provider_name VARCHAR(100) NOT NULL,
    content TEXT NOT NULL,
    confidence DECIMAL(3,2) NOT NULL DEFAULT 0.0,
    tokens_used INTEGER DEFAULT 0,
    response_time BIGINT DEFAULT 0,
    finish_reason VARCHAR(50) DEFAULT 'stop',
    metadata JSONB DEFAULT '{}',
    selected BOOLEAN DEFAULT FALSE,
    selection_score DECIMAL(5,2) DEFAULT 0.0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create cognee_memories table
CREATE TABLE IF NOT EXISTS cognee_memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID REFERENCES user_sessions(id) ON DELETE CASCADE,
    dataset_name VARCHAR(255) NOT NULL,
    content_type VARCHAR(50) DEFAULT 'text',
    content TEXT NOT NULL,
    vector_id VARCHAR(255),
    graph_nodes JSONB DEFAULT '{}',
    search_key VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);
CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_expires_at ON user_sessions(expires_at);
CREATE INDEX IF NOT EXISTS idx_user_sessions_session_token ON user_sessions(session_token);
CREATE INDEX IF NOT EXISTS idx_llm_providers_name ON llm_providers(name);
CREATE INDEX IF NOT EXISTS idx_llm_providers_enabled ON llm_providers(enabled);
CREATE INDEX IF NOT EXISTS idx_llm_requests_session_id ON llm_requests(session_id);
CREATE INDEX IF NOT EXISTS idx_llm_requests_user_id ON llm_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_llm_requests_status ON llm_requests(status);
CREATE INDEX IF NOT EXISTS idx_llm_responses_request_id ON llm_responses(request_id);
CREATE INDEX IF NOT EXISTS idx_llm_responses_provider_id ON llm_responses(provider_id);
CREATE INDEX IF NOT EXISTS idx_llm_responses_selected ON llm_responses(selected);
CREATE INDEX IF NOT EXISTS idx_cognee_memories_session_id ON cognee_memories(session_id);
CREATE INDEX IF NOT EXISTS idx_cognee_memories_dataset_name ON cognee_memories(dataset_name);
CREATE INDEX IF NOT EXISTS idx_cognee_memories_search_key ON cognee_memories(search_key);

-- Insert test user for development
INSERT INTO users (username, email, password_hash, api_key, role)
VALUES (
    'testuser',
    'test@example.com',
    '$argon2id$v=19$m=65536,t=1,p=4$test_salt$test_hash',
    'sk-test-api-key-for-development',
    'user'
) ON CONFLICT (username) DO NOTHING;

-- Insert test session
INSERT INTO user_sessions (user_id, session_token, expires_at, metadata)
SELECT
    id,
    'test-session-token-for-development',
    NOW() + INTERVAL '24 hours',
    '{"test": true}'
FROM users
WHERE username = 'testuser'
ON CONFLICT (session_token) DO NOTHING;