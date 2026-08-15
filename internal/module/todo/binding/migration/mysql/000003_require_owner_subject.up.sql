ALTER TABLE todos MODIFY COLUMN owner_subject VARCHAR(255) NOT NULL;
CREATE INDEX idx_todos_owner_created_at ON todos (owner_subject, created_at);
