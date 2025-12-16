# Database Migrations

This directory contains SQL migration files for the Minion Agent Framework database schema.

## Prerequisites

- PostgreSQL 14+ with the following extensions:
  - `uuid-ossp` (for UUID generation)
  - `pgvector` (for vector similarity search)

### Installing pgvector

**macOS (using Homebrew):**
```bash
brew install pgvector
```

**Ubuntu/Debian:**
```bash
sudo apt install postgresql-14-pgvector
```

**From source:**
```bash
cd /tmp
git clone --branch v0.5.1 https://github.com/pgvector/pgvector.git
cd pgvector
make
make install  # may need sudo
```

## Migration Files

Migrations follow the naming convention: `{version}_{name}.{direction}.sql`

- `001_initial_schema.up.sql` - Creates all tables, indexes, and functions
- `001_initial_schema.down.sql` - Rolls back the initial schema

## Running Migrations

### Option 1: Using psql (Manual)

**Apply migrations:**
```bash
psql -U minion -d minion -f migrations/001_initial_schema.up.sql
```

**Rollback migrations:**
```bash
psql -U minion -d minion -f migrations/001_initial_schema.down.sql
```

### Option 2: Using golang-migrate

**Install golang-migrate:**
```bash
brew install golang-migrate
```

**Apply all migrations:**
```bash
migrate -path migrations -database "postgresql://minion:minion_secret@localhost:5432/minion?sslmode=disable" up
```

**Rollback last migration:**
```bash
migrate -path migrations -database "postgresql://minion:minion_secret@localhost:5432/minion?sslmode=disable" down 1
```

### Option 3: Using the Makefile

```bash
make migrate-up
make migrate-down
```

## Database Setup

### 1. Create Database and User

```sql
CREATE USER minion WITH PASSWORD 'minion_secret';
CREATE DATABASE minion OWNER minion;
\c minion
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgvector";
GRANT ALL PRIVILEGES ON DATABASE minion TO minion;
GRANT ALL ON SCHEMA public TO minion;
```

### 2. Run Migrations

```bash
make migrate-up
```

### 3. Verify Schema

```bash
psql -U minion -d minion -c "\dt"
```

You should see:
- agents
- sessions
- memories
- activities
- metrics
- evaluations
- feedback
- tools
- agent_tools

## Schema Overview

### Core Tables

- **agents** - Agent configurations and definitions
- **sessions** - Conversation sessions with history and working memory
- **memories** - Long-term memories with vector embeddings for semantic search

### Observability Tables

- **activities** - Audit log of all agent actions and executions
- **metrics** - Time-series metrics for monitoring
- **evaluations** - Results from automated agent evaluation runs

### Improvement Tables

- **feedback** - Production feedback for the continuous improvement loop

### Registry Tables

- **tools** - Registry of available tools
- **agent_tools** - Many-to-many relationship between agents and tools

## Key Features

### 1. Vector Similarity Search

The `memories` table includes a `vector(1536)` column with HNSW index for fast semantic search:

```sql
-- Find similar memories
SELECT * FROM memories
WHERE agent_id = 'some-uuid'
ORDER BY embedding <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;
```

### 2. Automatic Timestamp Updates

All tables with `updated_at` columns have triggers that automatically update the timestamp on row changes.

### 3. Session Cleanup

A built-in function `cleanup_expired_sessions()` removes expired sessions:

```sql
SELECT cleanup_expired_sessions();
```

### 4. Summary Views

- **agent_stats** - Aggregated statistics per agent
- **recent_feedback_summary** - Feedback summary for the last 7 days

```sql
SELECT * FROM agent_stats WHERE agent_id = 'some-uuid';
SELECT * FROM recent_feedback_summary;
```

## Data Retention Policies

Consider setting up periodic cleanup jobs:

```sql
-- Delete old activities (older than 90 days)
DELETE FROM activities WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '90 days';

-- Delete old metrics (older than 30 days)
DELETE FROM metrics WHERE timestamp < CURRENT_TIMESTAMP - INTERVAL '30 days';

-- Archive closed sessions (older than 7 days)
UPDATE sessions SET status = 'archived'
WHERE status = 'closed' AND updated_at < CURRENT_TIMESTAMP - INTERVAL '7 days';
```

## Performance Tuning

### Recommended PostgreSQL Settings

```conf
# Memory
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 16MB

# Connections
max_connections = 100

# Query Planning
random_page_cost = 1.1  # For SSD
effective_io_concurrency = 200
```

### Index Maintenance

```sql
-- Reindex periodically for optimal performance
REINDEX TABLE memories;
REINDEX TABLE activities;

-- Analyze tables for query planning
ANALYZE agents;
ANALYZE sessions;
ANALYZE memories;
```

## Troubleshooting

### pgvector not found

```
ERROR:  type "vector" does not exist
```

**Solution:** Install pgvector extension (see Prerequisites above)

### Permission denied

```
ERROR:  permission denied for schema public
```

**Solution:** Grant proper permissions:
```sql
GRANT ALL ON SCHEMA public TO minion;
GRANT ALL ON ALL TABLES IN SCHEMA public TO minion;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO minion;
```

### Connection refused

```
ERROR:  could not connect to server: Connection refused
```

**Solution:** Ensure PostgreSQL is running:
```bash
# macOS
brew services start postgresql

# Linux
sudo systemctl start postgresql
```

## Development vs Production

### Development

- Use `sslmode=disable` for local development
- Smaller connection pool
- More verbose logging

### Production

- Use `sslmode=require` or `sslmode=verify-full`
- Larger connection pool (25-50 connections)
- Enable connection pooling (PgBouncer)
- Set up automated backups
- Enable point-in-time recovery (PITR)
- Monitor with pg_stat_statements

## Backup and Restore

### Backup

```bash
pg_dump -U minion -d minion > backup.sql
```

### Restore

```bash
psql -U minion -d minion < backup.sql
```

## Next Steps

After running migrations:

1. Verify all tables exist: `\dt`
2. Check extensions: `\dx`
3. Test vector search: Try inserting and querying a memory with an embedding
4. Run the application and verify connectivity
