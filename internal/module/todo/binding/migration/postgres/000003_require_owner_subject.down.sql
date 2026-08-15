DROP INDEX IF EXISTS idx_todos_owner_created_at;
ALTER TABLE todos ALTER COLUMN owner_subject DROP NOT NULL;
