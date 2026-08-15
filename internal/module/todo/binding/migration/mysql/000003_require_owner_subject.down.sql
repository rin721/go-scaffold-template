DROP INDEX idx_todos_owner_created_at ON todos;
ALTER TABLE todos MODIFY COLUMN owner_subject VARCHAR(255) NULL;
