DROP INDEX IF EXISTS idx_todos_status_created_at;
CREATE TABLE todos_owner_contract (
    id TEXT NOT NULL PRIMARY KEY,
    title TEXT NOT NULL,
    status TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    completed_at DATETIME NULL,
    version INTEGER NOT NULL,
    owner_subject TEXT NOT NULL
);
INSERT INTO todos_owner_contract (id, title, status, created_at, updated_at, completed_at, version, owner_subject)
SELECT id, title, status, created_at, updated_at, completed_at, version, owner_subject FROM todos;
DROP TABLE todos;
ALTER TABLE todos_owner_contract RENAME TO todos;
CREATE INDEX idx_todos_status_created_at ON todos (status, created_at);
CREATE INDEX idx_todos_owner_created_at ON todos (owner_subject, created_at);
