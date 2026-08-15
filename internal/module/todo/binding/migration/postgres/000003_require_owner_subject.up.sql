ALTER TABLE todos ALTER COLUMN owner_subject SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_todos_owner_created_at ON todos (owner_subject, created_at);
