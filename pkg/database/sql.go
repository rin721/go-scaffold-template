package database

import "gorm.io/gorm"

func quote(db *gorm.DB, value string) string {
	statement := &gorm.Statement{DB: db}
	return statement.Quote(value)
}
