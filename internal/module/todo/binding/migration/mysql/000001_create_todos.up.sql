CREATE TABLE IF NOT EXISTS todos (
    id VARCHAR(36) NOT NULL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    status VARCHAR(16) NOT NULL,
    created_at DATETIME(6) NOT NULL,
    updated_at DATETIME(6) NOT NULL,
    completed_at DATETIME(6) NULL,
    version BIGINT UNSIGNED NOT NULL,
    INDEX idx_todos_status_created_at (status, created_at)
) ENGINE=InnoDB;
