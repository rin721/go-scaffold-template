CREATE TABLE IF NOT EXISTS todos (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME NULL,
    version INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_todos_status_created_at ON todos (status, created_at);
