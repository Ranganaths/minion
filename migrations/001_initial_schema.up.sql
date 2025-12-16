-- Minion Agent Framework - Initial Database Schema
-- This schema supports all production features including agents, sessions, memory, and observability

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgvector";  -- For semantic search in memories

-- ============================================================================
-- AGENTS TABLE
-- ============================================================================
CREATE TABLE agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    personality TEXT,
    language VARCHAR(10) DEFAULT 'en',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',

    -- LLM Configuration
    llm_provider VARCHAR(50),
    llm_model VARCHAR(100),
    temperature DECIMAL(3, 2),
    max_tokens INTEGER,

    -- Behavior
    behavior_name VARCHAR(100),

    -- Capabilities
    capabilities JSONB DEFAULT '[]'::jsonb,

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT agents_status_check CHECK (status IN ('draft', 'active', 'inactive', 'archived')),
    CONSTRAINT agents_temperature_check CHECK (temperature >= 0 AND temperature <= 2.0)
);

CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_created_at ON agents(created_at DESC);
CREATE INDEX idx_agents_name ON agents(name);

-- ============================================================================
-- SESSIONS TABLE
-- ============================================================================
CREATE TABLE sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id VARCHAR(255),
    status VARCHAR(20) NOT NULL DEFAULT 'active',

    -- Conversation history (stored as JSON array)
    history JSONB DEFAULT '[]'::jsonb,

    -- Working memory/scratchpad
    workspace JSONB DEFAULT '{}'::jsonb,

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,

    -- Constraints
    CONSTRAINT sessions_status_check CHECK (status IN ('active', 'closed', 'expired', 'archived'))
);

CREATE INDEX idx_sessions_agent_id ON sessions(agent_id);
CREATE INDEX idx_sessions_user_id ON sessions(user_id);
CREATE INDEX idx_sessions_status ON sessions(status);
CREATE INDEX idx_sessions_created_at ON sessions(created_at DESC);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

-- ============================================================================
-- MEMORIES TABLE
-- ============================================================================
CREATE TABLE memories (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    user_id VARCHAR(255),

    -- Memory content
    key VARCHAR(255) NOT NULL,
    value TEXT NOT NULL,
    type VARCHAR(20) NOT NULL DEFAULT 'fact',
    source VARCHAR(255),  -- session_id or 'manual'

    -- Vector embedding for semantic search
    embedding vector(1536),  -- OpenAI ada-002 dimension

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Usage tracking
    access_count INTEGER DEFAULT 0,
    last_accessed TIMESTAMP WITH TIME ZONE,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT memories_type_check CHECK (type IN ('fact', 'preference', 'context', 'skill')),

    -- Unique constraint: one key per agent+user combination
    CONSTRAINT memories_unique_key UNIQUE (agent_id, user_id, key)
);

CREATE INDEX idx_memories_agent_id ON memories(agent_id);
CREATE INDEX idx_memories_user_id ON memories(user_id);
CREATE INDEX idx_memories_type ON memories(type);
CREATE INDEX idx_memories_created_at ON memories(created_at DESC);
CREATE INDEX idx_memories_access_count ON memories(access_count DESC);

-- Vector similarity search index (using HNSW for performance)
CREATE INDEX idx_memories_embedding ON memories USING hnsw (embedding vector_cosine_ops);

-- ============================================================================
-- ACTIVITIES TABLE (Audit Log)
-- ============================================================================
CREATE TABLE activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,

    -- Activity details
    action VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'success',

    -- Request/Response
    input JSONB,
    output JSONB,
    error TEXT,

    -- Performance metrics
    duration_ms INTEGER,
    token_count INTEGER,
    cost DECIMAL(10, 6),

    -- Tools used
    tools_used JSONB DEFAULT '[]'::jsonb,

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT activities_status_check CHECK (status IN ('success', 'failure', 'partial'))
);

CREATE INDEX idx_activities_agent_id ON activities(agent_id);
CREATE INDEX idx_activities_session_id ON activities(session_id);
CREATE INDEX idx_activities_action ON activities(action);
CREATE INDEX idx_activities_status ON activities(status);
CREATE INDEX idx_activities_created_at ON activities(created_at DESC);

-- ============================================================================
-- METRICS TABLE
-- ============================================================================
CREATE TABLE metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Metric name and type
    metric_name VARCHAR(100) NOT NULL,
    metric_type VARCHAR(20) NOT NULL,  -- counter, gauge, histogram

    -- Value
    value DECIMAL(20, 6) NOT NULL,

    -- Labels (for Prometheus-style metrics)
    labels JSONB DEFAULT '{}'::jsonb,

    -- Timestamp
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT metrics_type_check CHECK (metric_type IN ('counter', 'gauge', 'histogram'))
);

CREATE INDEX idx_metrics_agent_id ON metrics(agent_id);
CREATE INDEX idx_metrics_name ON metrics(metric_name);
CREATE INDEX idx_metrics_timestamp ON metrics(timestamp DESC);
CREATE INDEX idx_metrics_labels ON metrics USING gin (labels);

-- ============================================================================
-- EVALUATIONS TABLE
-- ============================================================================
CREATE TABLE evaluations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Evaluation metadata
    test_case_id VARCHAR(255),
    evaluator_type VARCHAR(50) NOT NULL,  -- effectiveness, efficiency, robustness, safety

    -- Input/Output
    input JSONB NOT NULL,
    expected_output JSONB,
    actual_output JSONB,

    -- Scores (0-1 scale)
    score DECIMAL(5, 4),
    effectiveness_score DECIMAL(5, 4),
    efficiency_score DECIMAL(5, 4),
    robustness_score DECIMAL(5, 4),
    safety_score DECIMAL(5, 4),

    -- Details
    passed BOOLEAN,
    feedback TEXT,
    details JSONB DEFAULT '{}'::jsonb,

    -- Performance
    duration_ms INTEGER,
    token_count INTEGER,
    cost DECIMAL(10, 6),

    -- Version tracking
    agent_version VARCHAR(50),

    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    -- Constraints
    CONSTRAINT evaluations_score_check CHECK (score >= 0 AND score <= 1)
);

CREATE INDEX idx_evaluations_agent_id ON evaluations(agent_id);
CREATE INDEX idx_evaluations_evaluator_type ON evaluations(evaluator_type);
CREATE INDEX idx_evaluations_passed ON evaluations(passed);
CREATE INDEX idx_evaluations_created_at ON evaluations(created_at DESC);
CREATE INDEX idx_evaluations_test_case_id ON evaluations(test_case_id);

-- ============================================================================
-- FEEDBACK TABLE (Production Feedback Loop)
-- ============================================================================
CREATE TABLE feedback (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    session_id UUID REFERENCES sessions(id) ON DELETE SET NULL,
    activity_id UUID REFERENCES activities(id) ON DELETE SET NULL,

    -- Feedback type
    feedback_type VARCHAR(50) NOT NULL,  -- failure, success, improvement, user_reported
    severity VARCHAR(20),  -- low, medium, high, critical

    -- Content
    title VARCHAR(255),
    description TEXT,

    -- Classification
    category VARCHAR(100),
    tags JSONB DEFAULT '[]'::jsonb,

    -- Resolution
    status VARCHAR(20) DEFAULT 'open',
    resolution TEXT,
    test_case_generated BOOLEAN DEFAULT false,
    test_case_id VARCHAR(255),

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    resolved_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT feedback_status_check CHECK (status IN ('open', 'in_review', 'resolved', 'wont_fix', 'duplicate')),
    CONSTRAINT feedback_severity_check CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE INDEX idx_feedback_agent_id ON feedback(agent_id);
CREATE INDEX idx_feedback_session_id ON feedback(session_id);
CREATE INDEX idx_feedback_type ON feedback(feedback_type);
CREATE INDEX idx_feedback_status ON feedback(status);
CREATE INDEX idx_feedback_severity ON feedback(severity);
CREATE INDEX idx_feedback_created_at ON feedback(created_at DESC);

-- ============================================================================
-- TOOLS TABLE (Tool Registry)
-- ============================================================================
CREATE TABLE tools (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL UNIQUE,
    description TEXT NOT NULL,
    category VARCHAR(100),

    -- Tool specification
    input_schema JSONB NOT NULL,
    output_schema JSONB,

    -- Version and lifecycle
    version VARCHAR(50) NOT NULL DEFAULT '1.0.0',
    status VARCHAR(20) NOT NULL DEFAULT 'active',

    -- Metadata
    tags JSONB DEFAULT '[]'::jsonb,
    documentation TEXT,
    examples JSONB DEFAULT '[]'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Operational
    timeout_seconds INTEGER DEFAULT 30,
    retry_config JSONB,
    cost_per_call DECIMAL(10, 6) DEFAULT 0,

    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deprecated_at TIMESTAMP WITH TIME ZONE,

    -- Constraints
    CONSTRAINT tools_status_check CHECK (status IN ('active', 'deprecated', 'disabled'))
);

CREATE INDEX idx_tools_name ON tools(name);
CREATE INDEX idx_tools_category ON tools(category);
CREATE INDEX idx_tools_status ON tools(status);
CREATE INDEX idx_tools_tags ON tools USING gin (tags);

-- ============================================================================
-- AGENT_TOOLS (Many-to-Many Relationship)
-- ============================================================================
CREATE TABLE agent_tools (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tool_id UUID NOT NULL REFERENCES tools(id) ON DELETE CASCADE,

    -- Configuration
    enabled BOOLEAN DEFAULT true,
    config JSONB DEFAULT '{}'::jsonb,

    -- Timestamps
    added_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (agent_id, tool_id)
);

CREATE INDEX idx_agent_tools_agent_id ON agent_tools(agent_id);
CREATE INDEX idx_agent_tools_tool_id ON agent_tools(tool_id);

-- ============================================================================
-- FUNCTIONS
-- ============================================================================

-- Function to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for updated_at
CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_sessions_updated_at BEFORE UPDATE ON sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_memories_updated_at BEFORE UPDATE ON memories
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tools_updated_at BEFORE UPDATE ON tools
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Function to clean up expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    deleted_count INTEGER;
BEGIN
    WITH deleted AS (
        DELETE FROM sessions
        WHERE expires_at < CURRENT_TIMESTAMP
        AND status != 'archived'
        RETURNING *
    )
    SELECT COUNT(*) INTO deleted_count FROM deleted;

    RETURN deleted_count;
END;
$$ LANGUAGE plpgsql;

-- ============================================================================
-- VIEWS
-- ============================================================================

-- View for agent summary statistics
CREATE OR REPLACE VIEW agent_stats AS
SELECT
    a.id AS agent_id,
    a.name,
    a.status,
    COUNT(DISTINCT s.id) AS total_sessions,
    COUNT(DISTINCT CASE WHEN s.status = 'active' THEN s.id END) AS active_sessions,
    COUNT(DISTINCT act.id) AS total_activities,
    COUNT(DISTINCT CASE WHEN act.status = 'success' THEN act.id END) AS successful_activities,
    COUNT(DISTINCT CASE WHEN act.status = 'failure' THEN act.id END) AS failed_activities,
    AVG(act.duration_ms) AS avg_duration_ms,
    SUM(act.cost) AS total_cost,
    MAX(act.created_at) AS last_activity_at
FROM agents a
LEFT JOIN sessions s ON s.agent_id = a.id
LEFT JOIN activities act ON act.agent_id = a.id
GROUP BY a.id, a.name, a.status;

-- View for recent feedback summary
CREATE OR REPLACE VIEW recent_feedback_summary AS
SELECT
    a.id AS agent_id,
    a.name AS agent_name,
    f.feedback_type,
    f.severity,
    COUNT(*) AS count,
    MAX(f.created_at) AS most_recent
FROM feedback f
JOIN agents a ON f.agent_id = a.id
WHERE f.created_at > CURRENT_TIMESTAMP - INTERVAL '7 days'
GROUP BY a.id, a.name, f.feedback_type, f.severity
ORDER BY count DESC;

-- ============================================================================
-- INITIAL DATA
-- ============================================================================

COMMENT ON TABLE agents IS 'Stores agent configurations and definitions';
COMMENT ON TABLE sessions IS 'Stores conversation sessions with history and working memory';
COMMENT ON TABLE memories IS 'Stores long-term memories with vector embeddings for semantic search';
COMMENT ON TABLE activities IS 'Audit log of all agent actions and executions';
COMMENT ON TABLE metrics IS 'Time-series metrics for observability';
COMMENT ON TABLE evaluations IS 'Results from agent evaluation runs';
COMMENT ON TABLE feedback IS 'Production feedback for continuous improvement';
COMMENT ON TABLE tools IS 'Registry of available tools';
COMMENT ON TABLE agent_tools IS 'Association between agents and their enabled tools';
