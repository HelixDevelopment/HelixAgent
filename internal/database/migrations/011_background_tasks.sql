-- Migration: 011_background_tasks
-- Description: Create tables for background task execution system
-- Date: 2026-01-12

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Task status enum
DO $$ BEGIN
    CREATE TYPE task_status AS ENUM (
        'pending',      -- Task created, waiting in queue
        'queued',       -- Task assigned to worker pool
        'running',      -- Task currently executing
        'paused',       -- Task paused by user/system
        'completed',    -- Task finished successfully
        'failed',       -- Task failed with error
        'stuck',        -- Task detected as stuck
        'cancelled',    -- Task cancelled by user
        'dead_letter'   -- Task moved to dead-letter queue
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Task priority enum
DO $$ BEGIN
    CREATE TYPE task_priority AS ENUM (
        'critical',     -- P0: Execute immediately
        'high',         -- P1: Execute next
        'normal',       -- P2: Default priority
        'low',          -- P3: Execute when resources available
        'background'    -- P4: Execute during low load
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Main background tasks table
CREATE TABLE IF NOT EXISTS background_tasks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),

    -- Task identification
    task_type VARCHAR(100) NOT NULL,
    task_name VARCHAR(255) NOT NULL,
    correlation_id VARCHAR(255),
    parent_task_id UUID REFERENCES background_tasks(id) ON DELETE SET NULL,

    -- Task configuration
    payload JSONB NOT NULL DEFAULT '{}',
    config JSONB NOT NULL DEFAULT '{}',
    priority task_priority NOT NULL DEFAULT 'normal',

    -- State management
    status task_status NOT NULL DEFAULT 'pending',
    progress DECIMAL(5,2) DEFAULT 0.0,
    progress_message TEXT,
    checkpoint JSONB,

    -- Retry configuration
    max_retries INTEGER DEFAULT 3,
    retry_count INTEGER DEFAULT 0,
    retry_delay_seconds INTEGER DEFAULT 60,
    last_error TEXT,
    error_history JSONB DEFAULT '[]',

    -- Execution tracking
    worker_id VARCHAR(100),
    process_pid INTEGER,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    deadline TIMESTAMP WITH TIME ZONE,

    -- Resource requirements
    required_cpu_cores INTEGER DEFAULT 1,
    required_memory_mb INTEGER DEFAULT 512,
    estimated_duration_seconds INTEGER,
    actual_duration_seconds INTEGER,

    -- Notification configuration
    notification_config JSONB DEFAULT '{}',

    -- User association
    user_id UUID,
    session_id UUID,
    tags JSONB DEFAULT '[]',
    metadata JSONB DEFAULT '{}',

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    scheduled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    -- Soft delete
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Dead-letter queue for failed tasks
CREATE TABLE IF NOT EXISTS background_tasks_dead_letter (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    original_task_id UUID NOT NULL,
    task_data JSONB NOT NULL,
    failure_reason TEXT NOT NULL,
    failure_count INTEGER DEFAULT 1,
    moved_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    reprocess_after TIMESTAMP WITH TIME ZONE,
    reprocessed BOOLEAN DEFAULT FALSE
);

-- Task execution history for auditing
CREATE TABLE IF NOT EXISTS task_execution_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES background_tasks(id) ON DELETE CASCADE,
    event_type VARCHAR(50) NOT NULL,
    event_data JSONB DEFAULT '{}',
    worker_id VARCHAR(100),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Resource snapshots for monitoring and stuck detection
CREATE TABLE IF NOT EXISTS task_resource_snapshots (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL REFERENCES background_tasks(id) ON DELETE CASCADE,

    -- CPU metrics
    cpu_percent DECIMAL(5,2),
    cpu_user_time DECIMAL(12,4),
    cpu_system_time DECIMAL(12,4),

    -- Memory metrics
    memory_rss_bytes BIGINT,
    memory_vms_bytes BIGINT,
    memory_percent DECIMAL(5,2),

    -- I/O metrics
    io_read_bytes BIGINT,
    io_write_bytes BIGINT,
    io_read_count BIGINT,
    io_write_count BIGINT,

    -- Network metrics
    net_bytes_sent BIGINT,
    net_bytes_recv BIGINT,
    net_connections INTEGER,

    -- File descriptors
    open_files INTEGER,
    open_fds INTEGER,

    -- Process state
    process_state VARCHAR(20),
    thread_count INTEGER,

    sampled_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Webhook delivery tracking
CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID REFERENCES background_tasks(id) ON DELETE SET NULL,
    webhook_url TEXT NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'pending',
    attempts INTEGER DEFAULT 0,
    last_attempt_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT,
    response_code INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    delivered_at TIMESTAMP WITH TIME ZONE
);

-- Indexes for efficient querying
CREATE INDEX IF NOT EXISTS idx_tasks_status ON background_tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_priority_status ON background_tasks(priority, status, scheduled_at);
CREATE INDEX IF NOT EXISTS idx_tasks_worker ON background_tasks(worker_id) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_tasks_user ON background_tasks(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_correlation ON background_tasks(correlation_id) WHERE correlation_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_scheduled ON background_tasks(scheduled_at) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tasks_heartbeat ON background_tasks(last_heartbeat) WHERE status = 'running';
CREATE INDEX IF NOT EXISTS idx_tasks_deadline ON background_tasks(deadline) WHERE deadline IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_type ON background_tasks(task_type);
CREATE INDEX IF NOT EXISTS idx_tasks_created ON background_tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_not_deleted ON background_tasks(id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_task_history_task_id ON task_execution_history(task_id);
CREATE INDEX IF NOT EXISTS idx_task_history_event_type ON task_execution_history(event_type);
CREATE INDEX IF NOT EXISTS idx_task_history_created ON task_execution_history(created_at DESC);

CREATE INDEX IF NOT EXISTS idx_resource_snapshots_task ON task_resource_snapshots(task_id, sampled_at DESC);
CREATE INDEX IF NOT EXISTS idx_resource_snapshots_recent ON task_resource_snapshots(sampled_at DESC);

CREATE INDEX IF NOT EXISTS idx_dead_letter_reprocess ON background_tasks_dead_letter(reprocess_after) WHERE NOT reprocessed;
CREATE INDEX IF NOT EXISTS idx_dead_letter_original ON background_tasks_dead_letter(original_task_id);

CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_task ON webhook_deliveries(task_id);
CREATE INDEX IF NOT EXISTS idx_webhook_deliveries_status ON webhook_deliveries(status) WHERE status != 'delivered';

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_background_task_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for automatic updated_at
DROP TRIGGER IF EXISTS trg_background_task_updated_at ON background_tasks;
CREATE TRIGGER trg_background_task_updated_at
    BEFORE UPDATE ON background_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_background_task_updated_at();

-- Function to get next task from queue with atomic dequeue
CREATE OR REPLACE FUNCTION dequeue_background_task(
    p_worker_id VARCHAR(100),
    p_max_cpu_cores INTEGER DEFAULT 0,
    p_max_memory_mb INTEGER DEFAULT 0
)
RETURNS TABLE (
    task_id UUID,
    task_type VARCHAR(100),
    task_name VARCHAR(255),
    payload JSONB,
    config JSONB,
    priority task_priority
) AS $$
DECLARE
    v_task_id UUID;
BEGIN
    -- Atomically select and update task with FOR UPDATE SKIP LOCKED
    UPDATE background_tasks
    SET
        status = 'running',
        worker_id = p_worker_id,
        started_at = NOW(),
        last_heartbeat = NOW()
    WHERE id = (
        SELECT bt.id
        FROM background_tasks bt
        WHERE bt.status = 'pending'
          AND bt.scheduled_at <= NOW()
          AND bt.deleted_at IS NULL
          AND (p_max_cpu_cores = 0 OR bt.required_cpu_cores <= p_max_cpu_cores)
          AND (p_max_memory_mb = 0 OR bt.required_memory_mb <= p_max_memory_mb)
        ORDER BY
            CASE bt.priority
                WHEN 'critical' THEN 0
                WHEN 'high' THEN 1
                WHEN 'normal' THEN 2
                WHEN 'low' THEN 3
                WHEN 'background' THEN 4
            END,
            bt.created_at ASC
        LIMIT 1
        FOR UPDATE SKIP LOCKED
    )
    RETURNING background_tasks.id INTO v_task_id;

    -- Return the task data if one was dequeued
    IF v_task_id IS NOT NULL THEN
        RETURN QUERY
        SELECT
            bt.id,
            bt.task_type,
            bt.task_name,
            bt.payload,
            bt.config,
            bt.priority
        FROM background_tasks bt
        WHERE bt.id = v_task_id;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to get stale tasks (for stuck detection)
CREATE OR REPLACE FUNCTION get_stale_tasks(
    p_heartbeat_threshold INTERVAL DEFAULT INTERVAL '5 minutes'
)
RETURNS TABLE (
    task_id UUID,
    task_type VARCHAR(100),
    worker_id VARCHAR(100),
    started_at TIMESTAMP WITH TIME ZONE,
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    time_since_heartbeat INTERVAL
) AS $$
BEGIN
    RETURN QUERY
    SELECT
        bt.id,
        bt.task_type,
        bt.worker_id,
        bt.started_at,
        bt.last_heartbeat,
        NOW() - bt.last_heartbeat AS time_since_heartbeat
    FROM background_tasks bt
    WHERE bt.status = 'running'
      AND bt.last_heartbeat < NOW() - p_heartbeat_threshold
      AND bt.deleted_at IS NULL;
END;
$$ LANGUAGE plpgsql;

-- Comment on tables
COMMENT ON TABLE background_tasks IS 'Main table for background task management with PostgreSQL-backed queue';
COMMENT ON TABLE background_tasks_dead_letter IS 'Dead-letter queue for tasks that failed after max retries';
COMMENT ON TABLE task_execution_history IS 'Audit trail of task state changes and events';
COMMENT ON TABLE task_resource_snapshots IS 'Time-series resource usage data for monitoring and stuck detection';
COMMENT ON TABLE webhook_deliveries IS 'Webhook notification delivery tracking with retry support';
