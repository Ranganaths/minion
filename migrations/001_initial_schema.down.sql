-- Rollback initial schema

-- Drop views
DROP VIEW IF EXISTS recent_feedback_summary;
DROP VIEW IF EXISTS agent_stats;

-- Drop functions
DROP FUNCTION IF EXISTS cleanup_expired_sessions();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables (in reverse order of creation to handle dependencies)
DROP TABLE IF EXISTS agent_tools;
DROP TABLE IF EXISTS tools;
DROP TABLE IF EXISTS feedback;
DROP TABLE IF EXISTS evaluations;
DROP TABLE IF EXISTS metrics;
DROP TABLE IF EXISTS activities;
DROP TABLE IF EXISTS memories;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS agents;

-- Drop extensions (commented out to avoid affecting other databases)
-- DROP EXTENSION IF EXISTS "pgvector";
-- DROP EXTENSION IF EXISTS "uuid-ossp";
