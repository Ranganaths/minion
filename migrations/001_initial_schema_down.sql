-- Rollback migration 001: Drop all tables and functions

-- Drop triggers
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
DROP TRIGGER IF EXISTS update_agent_state_updated_at ON agent_state;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_expired_dedup();

-- Drop indexes (will be dropped with tables, but explicit for clarity)
DROP INDEX IF EXISTS idx_tasks_status;
DROP INDEX IF EXISTS idx_tasks_assigned_to;
DROP INDEX IF EXISTS idx_tasks_created_by;
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP INDEX IF EXISTS idx_tasks_updated_at;
DROP INDEX IF EXISTS idx_tasks_completed_at;
DROP INDEX IF EXISTS idx_tasks_type;
DROP INDEX IF EXISTS idx_tasks_priority;
DROP INDEX IF EXISTS idx_tasks_status_created_at;
DROP INDEX IF EXISTS idx_tasks_assigned_status;

DROP INDEX IF EXISTS idx_progress_task_id;
DROP INDEX IF EXISTS idx_progress_agent_id;
DROP INDEX IF EXISTS idx_progress_recorded_at;
DROP INDEX IF EXISTS idx_progress_status;
DROP INDEX IF EXISTS idx_progress_task_recorded;

DROP INDEX IF EXISTS idx_agent_state_status;
DROP INDEX IF EXISTS idx_agent_state_role;
DROP INDEX IF EXISTS idx_agent_state_heartbeat;
DROP INDEX IF EXISTS idx_agent_state_status_heartbeat;

DROP INDEX IF EXISTS idx_message_dedup_expires;

-- Drop tables (in reverse dependency order)
DROP TABLE IF EXISTS task_progress;
DROP TABLE IF EXISTS message_dedup;
DROP TABLE IF EXISTS agent_state;
DROP TABLE IF EXISTS tasks;

-- Migration rollback complete
SELECT 'Migration 001: Schema rolled back successfully' AS status;
