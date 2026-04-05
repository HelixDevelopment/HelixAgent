-- Migration 002: Performance and Security Tables
-- Added as part of production readiness Phase 3

-- Feature flags table
CREATE TABLE IF NOT EXISTS feature_flags (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    enabled BOOLEAN NOT NULL DEFAULT false,
    description TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Performance baselines table
CREATE TABLE IF NOT EXISTS performance_baselines (
    id SERIAL PRIMARY KEY,
    metric_name VARCHAR(255) NOT NULL,
    package_name VARCHAR(255) NOT NULL,
    baseline_ns BIGINT NOT NULL,
    baseline_allocs BIGINT,
    baseline_bytes BIGINT,
    captured_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(metric_name, package_name)
);

-- Security scan history table
CREATE TABLE IF NOT EXISTS security_scan_history (
    id SERIAL PRIMARY KEY,
    tool_name VARCHAR(100) NOT NULL,
    scan_type VARCHAR(50) NOT NULL,
    findings_critical INTEGER DEFAULT 0,
    findings_high INTEGER DEFAULT 0,
    findings_medium INTEGER DEFAULT 0,
    findings_low INTEGER DEFAULT 0,
    findings_info INTEGER DEFAULT 0,
    scan_duration_ms BIGINT,
    report_path TEXT,
    scanned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Benchmark run results (complements in-memory storage)
CREATE TABLE IF NOT EXISTS benchmark_runs (
    id VARCHAR(100) PRIMARY KEY,
    benchmark_type VARCHAR(50) NOT NULL,
    provider_name VARCHAR(100) NOT NULL,
    model_name VARCHAR(100),
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    pass_rate REAL,
    average_score REAL,
    average_latency_ns BIGINT,
    total_tasks INTEGER,
    passed_tasks INTEGER,
    failed_tasks INTEGER,
    config JSONB,
    summary JSONB,
    started_at TIMESTAMP WITH TIME ZONE,
    ended_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_benchmark_runs_type ON benchmark_runs(benchmark_type);
CREATE INDEX idx_benchmark_runs_provider ON benchmark_runs(provider_name);
CREATE INDEX idx_security_scans_tool ON security_scan_history(tool_name);
CREATE INDEX idx_feature_flags_name ON feature_flags(name);
