DROP INDEX IF EXISTS idx_todos_owner_created_at;
DROP INDEX IF EXISTS idx_todos_status_created_at;
CREATE TABLE todos_nullable_owner (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME NULL,
    version INTEGER NOT NULL,
    owner_subject TEXT NULL
);
INSERT INTO todos_nullable_owner (id, title, status, created_at, updated_at, completed_at, version, owner_subject)
SELECT id, title, status, created_at, updated_at, completed_at, version, owner_subject FROM todos;
DROP TABLE todos;
ALTER TABLE todos_nullable_owner RENAME TO todos;
CREATE INDEX idx_todos_status_created_at ON todos (status, created_at);
