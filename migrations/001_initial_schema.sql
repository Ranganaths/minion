-- Migration 001: Initial schema for multi-agent system
-- Tasks and progress tracking tables

-- Tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(255) PRIMARY KEY,
    name VARCHAR(500) NOT NULL,
    description TEXT,
    type VARCHAR(100) NOT NULL,
    priority INTEGER NOT NULL DEFAULT 5,
    assigned_to VARCHAR(255),
    created_by VARCHAR(255) NOT NULL,
    dependencies JSONB DEFAULT '[]'::jsonb,
    input JSONB,
    output JSONB,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,

    -- Constraints
    CONSTRAINT valid_status CHECK (status IN (
        'pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled'
    )),
    CONSTRAINT valid_priority CHECK (priority BETWEEN 1 AND 10)
);

-- Task progress table
CREATE TABLE IF NOT EXISTS task_progress (
    id SERIAL PRIMARY KEY,
    task_id VARCHAR(255) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_progress_status CHECK (status IN (
        'pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled'
    ))
);

-- Agent state table (for worker management and auto-scaling)
CREATE TABLE IF NOT EXISTS agent_state (
    agent_id VARCHAR(255) PRIMARY KEY,
    role VARCHAR(50) NOT NULL,
    capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
    status VARCHAR(50) NOT NULL DEFAULT 'offline',
    priority INTEGER DEFAULT 5,
    metadata JSONB DEFAULT '{}'::jsonb,
    last_heartbeat TIMESTAMP NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_agent_status CHECK (status IN (
        'idle', 'busy', 'waiting', 'failed', 'offline'
    )),
    CONSTRAINT valid_role CHECK (role IN (
        'orchestrator', 'worker', 'specialist', 'monitor'
    ))
);

-- Message deduplication table
CREATE TABLE IF NOT EXISTS message_dedup (
    message_id VARCHAR(255) PRIMARY KEY,
    processed_at TIMESTAMP NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMP NOT NULL,
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Indexes for tasks table
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by ON tasks(created_by);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_updated_at ON tasks(updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_completed_at ON tasks(completed_at DESC) WHERE completed_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_type ON tasks(type);
CREATE INDEX IF NOT EXISTS idx_tasks_priority ON tasks(priority DESC);

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_status_created_at ON tasks(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_status ON tasks(assigned_to, status) WHERE assigned_to IS NOT NULL;

-- Indexes for task_progress table
CREATE INDEX IF NOT EXISTS idx_progress_task_id ON task_progress(task_id);
CREATE INDEX IF NOT EXISTS idx_progress_agent_id ON task_progress(agent_id);
CREATE INDEX IF NOT EXISTS idx_progress_recorded_at ON task_progress(recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_progress_status ON task_progress(status);
CREATE INDEX IF NOT EXISTS idx_progress_task_recorded ON task_progress(task_id, recorded_at DESC);

-- Indexes for agent_state table
CREATE INDEX IF NOT EXISTS idx_agent_state_status ON agent_state(status);
CREATE INDEX IF NOT EXISTS idx_agent_state_role ON agent_state(role);
CREATE INDEX IF NOT EXISTS idx_agent_state_heartbeat ON agent_state(last_heartbeat DESC);
CREATE INDEX IF NOT EXISTS idx_agent_state_status_heartbeat ON agent_state(status, last_heartbeat DESC);

-- Indexes for message_dedup table
CREATE INDEX IF NOT EXISTS idx_message_dedup_expires ON message_dedup(expires_at);

-- Comments for documentation
COMMENT ON TABLE tasks IS 'Main task ledger for multi-agent system';
COMMENT ON TABLE task_progress IS 'Progress tracking for tasks';
COMMENT ON TABLE agent_state IS 'State and health of agents';
COMMENT ON TABLE message_dedup IS 'Message deduplication tracking';

COMMENT ON COLUMN tasks.id IS 'Unique task identifier (UUID)';
COMMENT ON COLUMN tasks.dependencies IS 'Array of task IDs this task depends on';
COMMENT ON COLUMN tasks.input IS 'Task input data (flexible JSON)';
COMMENT ON COLUMN tasks.output IS 'Task result data (flexible JSON)';
COMMENT ON COLUMN tasks.metadata IS 'Additional task metadata (flexible JSON)';

-- Functions for automatic timestamp updates
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Triggers for automatic timestamp updates
CREATE TRIGGER update_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_state_updated_at
    BEFORE UPDATE ON agent_state
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Function to clean up expired dedup entries
CREATE OR REPLACE FUNCTION cleanup_expired_dedup()
RETURNS void AS $$
BEGIN
    DELETE FROM message_dedup WHERE expires_at < NOW();
END;
$$ LANGUAGE plpgsql;

-- Migration complete
SELECT 'Migration 001: Initial schema created successfully' AS status;
