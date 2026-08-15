package database

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// migrateForTest 只为 Repository 单元测试建立隔离表，不进入 production API。
func (c *gormClient) migrateForTest(ctx context.Context, schemas ...Schema) error {
	if c.unavailable() {
		return ErrClientUnavailable
	}
	resolvedByTable := make(map[string]resolvedSchema, len(schemas))
	ordered := make([]resolvedSchema, 0, len(schemas))
	for _, schema := range schemas {
		resolved, err := resolveSchema(schema)
		if err != nil {
			return err
		}
		resolvedByTable[resolved.Table] = resolved
		ordered = append(ordered, resolved)
	}
	if err := validateReferences(resolvedByTable); err != nil {
		return err
	}
	for _, resolved := range ordered {
		db := c.db.WithContext(ctx).Table(resolved.Table)
		if db.Dialector.Name() == "sqlite" && len(resolved.References) > 0 {
			if err := createSQLiteFixture(db, resolved, resolvedByTable); err != nil {
				return err
			}
		} else if err := db.Migrator().CreateTable(resolved.dynamicModel()); err != nil {
			return err
		}
		for _, index := range resolved.Indexes {
			if db.Migrator().HasIndex(resolved.dynamicModel(), index.Name) {
				continue
			}
			if err := db.Migrator().CreateIndex(resolved.dynamicModel(), index.Name); err != nil {
				return err
			}
		}
	}
	for _, resolved := range ordered {
		if c.db.Dialector.Name() == "sqlite" {
			continue
		}
		for _, reference := range resolved.References {
			if err := addFixtureReference(c.db.WithContext(ctx), resolved, reference, resolvedByTable); err != nil {
				return err
			}
		}
	}
	return nil
}

func addFixtureReference(db *gorm.DB, schema resolvedSchema, reference Reference, schemas map[string]resolvedSchema) error {
	field := schema.fields[reference.Field]
	target := schemas[reference.Table]
	targetField := target.fields[reference.ReferenceField]
	statement := "ALTER TABLE " + fixtureQuote(db, schema.Table) +
		" ADD CONSTRAINT " + fixtureQuote(db, "fk_"+schema.Table+"_"+field.Column) +
		" FOREIGN KEY (" + fixtureQuote(db, field.Column) + ") REFERENCES " +
		fixtureQuote(db, reference.Table) + " (" + fixtureQuote(db, targetField.Column) + ")"
	if reference.OnUpdate != "" {
		statement += " ON UPDATE " + string(reference.OnUpdate)
	}
	if reference.OnDelete != "" {
		statement += " ON DELETE " + string(reference.OnDelete)
	}
	return db.Exec(statement).Error
}

func createSQLiteFixture(db *gorm.DB, schema resolvedSchema, schemas map[string]resolvedSchema) error {
	columns := make([]string, 0, len(schema.Fields)+len(schema.References))
	for _, field := range schema.Fields {
		definition := fixtureQuote(db, field.Column) + " " + sqliteFixtureType(field)
		if field.PrimaryKey {
			definition += " PRIMARY KEY"
			if field.AutoIncrement {
				definition += " AUTOINCREMENT"
			}
		}
		if !field.Nullable {
			definition += " NOT NULL"
		}
		columns = append(columns, definition)
	}
	for _, reference := range schema.References {
		field := schema.fields[reference.Field]
		target := schemas[reference.Table]
		targetField := target.fields[reference.ReferenceField]
		definition := "CONSTRAINT " + fixtureQuote(db, "fk_"+schema.Table+"_"+field.Column) +
			" FOREIGN KEY (" + fixtureQuote(db, field.Column) + ") REFERENCES " +
			fixtureQuote(db, reference.Table) + " (" + fixtureQuote(db, targetField.Column) + ")"
		if reference.OnUpdate != "" {
			definition += " ON UPDATE " + string(reference.OnUpdate)
		}
		if reference.OnDelete != "" {
			definition += " ON DELETE " + string(reference.OnDelete)
		}
		columns = append(columns, definition)
	}
	statement := "CREATE TABLE " + fixtureQuote(db, schema.Table) + " (" + strings.Join(columns, ", ") + ")"
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("create SQLite test fixture %q: %w", schema.Table, err)
	}
	return nil
}

func sqliteFixtureType(field Field) string {
	switch field.Type {
	case FieldBool, FieldInt, FieldInt64, FieldUint, FieldUint64:
		return "INTEGER"
	case FieldFloat64:
		return "REAL"
	case FieldBytes:
		return "BLOB"
	case FieldTime:
		return "DATETIME"
	default:
		return "TEXT"
	}
}

func fixtureQuote(db *gorm.DB, value string) string {
	statement := &gorm.Statement{DB: db}
	return statement.Quote(value)
}
