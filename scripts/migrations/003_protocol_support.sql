-- Protocol Support Migration for Enhanced LLM Power Features
-- Extends existing models_metadata table to support MCP, LSP, ACP, Embeddings
-- Adds new tables for protocol-specific data and caching
-- Creates unified management system for all protocols

-- Run this migration to extend database schema:
-- psql -f scripts/migrations/003_protocol_support.sql

BEGIN;

-- Extend models_metadata table for protocol support
ALTER TABLE models_metadata 
ADD COLUMN IF NOT EXISTS protocol_support JSONB DEFAULT '[]'::json[],
ADD COLUMN IF NOT EXISTS mcp_server_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS lsp_server_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS acp_server_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS embedding_provider VARCHAR(50) DEFAULT 'pgvector',
ADD COLUMN IF NOT EXISTS protocol_config JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS protocol_last_sync TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;

-- MCP Server Configuration Table
CREATE TABLE IF NOT EXISTS mcp_servers (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('local', 'remote')),
    command TEXT,
    url TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    tools JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_sync TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- LSP Server Configuration Table
CREATE TABLE IF NOT EXISTS lsp_servers (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    language VARCHAR(50) NOT NULL,
    command VARCHAR(500) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    workspace VARCHAR(1000) DEFAULT '/workspace',
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_sync TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- ACP Server Configuration Table
CREATE TABLE IF NOT EXISTS acp_servers (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('local', 'remote')),
    url TEXT,
    enabled BOOLEAN NOT NULL DEFAULT true,
    tools JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_sync TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Embedding Configuration Table
CREATE TABLE IF NOT EXISTS embedding_config (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL DEFAULT 'pgvector',
    model VARCHAR(100) NOT NULL DEFAULT 'text-embedding-ada-002',
    dimension INTEGER NOT NULL DEFAULT 1536,
    api_endpoint TEXT,
    api_key TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Vector Document Table for Semantic Search
CREATE TABLE IF NOT EXISTS vector_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb,
    embedding_id UUID,
    embedding VECTOR(1536),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    embedding_provider VARCHAR(50) DEFAULT 'pgvector',
    search_vector VECTOR(1536),
    CONSTRAINT embedding_search_idx_cosine_similarity_cosine_op_idx INCLUDE (search_vector) WITH (cosine_distance >= 0.7)
);

-- Protocol Cache Table
CREATE TABLE IF NOT EXISTS protocol_cache (
    cache_key VARCHAR(255) PRIMARY KEY,
    cache_data JSONB NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Protocol Metrics Table
CREATE TABLE IF NOT EXISTS protocol_metrics (
    id SERIAL PRIMARY KEY,
    protocol_type VARCHAR(20) NOT NULL CHECK (protocol_type IN ('mcp', 'lsp', 'acp', 'embedding')),
    server_id VARCHAR(255),
    operation VARCHAR(100) NOT NULL,
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'error', 'timeout')),
    duration_ms INTEGER,
    error_message TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for Performance
CREATE INDEX IF NOT EXISTS idx_models_metadata_protocol_support ON models_metadata USING GIN (protocol_support);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_enabled ON mcp_servers USING btree (enabled);
CREATE INDEX IF NOT EXISTS idx_lsp_servers_enabled ON lsp_servers USING btree (enabled);
CREATE INDEX IF NOT EXISTS idx_acp_servers_enabled ON acp_servers USING btree (enabled);
CREATE INDEX IF NOT EXISTS idx_vector_documents_embedding_search_vector ON vector_documents USING GIN (embedding_search_vector);
CREATE INDEX IF NOT EXISTS idx_vector_documents_embedding_provider ON vector_documents USING btree (embedding_provider);
CREATE INDEX IF NOT EXISTS idx_protocol_cache_expires_at ON protocol_cache USING btree (expires_at);
CREATE INDEX IF NOT EXISTS idx_protocol_metrics_protocol_type ON protocol_metrics USING btree (protocol_type);
CREATE INDEX IF NOT EXISTS idx_protocol_metrics_created_at ON protocol_metrics USING btree (created_at);

-- Insert default configuration data
INSERT INTO embedding_config (provider, model, dimension, api_endpoint, api_key, created_at, updated_at) 
VALUES ('pgvector', 'text-embedding-ada-002', 1536, 'http://localhost:7432/vector', 'your-api-key-here', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);

-- Add comment about protocol support
COMMENT ON TABLE models_metadata IS 'Protocol support for MCP, LSP, ACP, and Embeddings integration';

-- Enable pgvector extension (required for vector operations)
CREATE EXTENSION IF NOT EXISTS pgvector;

COMMIT;