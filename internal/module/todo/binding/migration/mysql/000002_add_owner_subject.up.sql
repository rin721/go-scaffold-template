ALTER TABLE todos ADD COLUMN owner_subject VARCHAR(255) NULL;
UPDATE todos SET owner_subject = 'migration:unassigned' WHERE owner_subject IS NULL OR owner_subject = '';
