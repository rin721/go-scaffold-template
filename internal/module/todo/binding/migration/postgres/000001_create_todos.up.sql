CREATE TABLE IF NOT EXISTS todos (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ NULL,
    version BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_todos_status_created_at ON todos (status, created_at);
