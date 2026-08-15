package database

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

func (c *gormClient) Migrate(ctx context.Context, schemas ...Schema) error {
	if c.unavailable() {
		return ErrClientUnavailable
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	resolvedSchemas := make([]resolvedSchema, 0, len(schemas))
	byTable := make(map[string]resolvedSchema, len(schemas))
	for _, schema := range schemas {
		resolved, err := resolveSchema(schema)
		if err != nil {
			return err
		}
		if _, exists := byTable[resolved.Table]; exists {
			return fmt.Errorf("%w: duplicate schema table %q", ErrInvalidSchema, resolved.Table)
		}
		resolvedSchemas = append(resolvedSchemas, resolved)
		byTable[resolved.Table] = resolved
	}
	if err := validateReferences(byTable); err != nil {
		return err
	}
	globalIndexes := make(map[string]string)
	for _, resolved := range resolvedSchemas {
		for _, index := range resolved.Indexes {
			if owner, exists := globalIndexes[index.Name]; exists {
				return fmt.Errorf("%w: index %q is shared by tables %q and %q", ErrInvalidSchema, index.Name, owner, resolved.Table)
			}
			globalIndexes[index.Name] = resolved.Table
		}
	}
	// SQLite 不能用 ALTER TABLE 追加外键。先完成整批预检，避免已增加列或索引后才失败。
	if c.db.Dialector.Name() == "sqlite" {
		for _, resolved := range resolvedSchemas {
			if !c.db.Migrator().HasTable(resolved.Table) {
				continue
			}
			for _, reference := range resolved.References {
				constraint := "fk_" + resolved.Table + "_" + resolved.fields[reference.Field].Column
				if !c.db.Migrator().HasConstraint(resolved.Table, constraint) {
					return fmt.Errorf("%w: sqlite cannot add missing constraint %q to an existing table", ErrInvalidSchema, constraint)
				}
			}
		}
	}
	for _, resolved := range resolvedSchemas {
		model := resolved.dynamicModel()
		db := c.db.WithContext(ctx).Table(resolved.Table)
		tableExists := db.Migrator().HasTable(resolved.Table)
		if !tableExists {
			if db.Dialector.Name() == "sqlite" && len(resolved.References) > 0 {
				if err := createSQLiteTable(db, resolved, byTable); err != nil {
					return err
				}
			} else if err := db.Migrator().CreateTable(model); err != nil {
				return fmt.Errorf("create table %q: %w", resolved.Table, translateError(err))
			}
		} else {
			for _, field := range resolved.Fields {
				if db.Migrator().HasColumn(model, field.Name) {
					continue
				}
				if err := db.Migrator().AddColumn(model, field.Name); err != nil {
					return fmt.Errorf("add column %q.%q: %w", resolved.Table, field.Column, translateError(err))
				}
			}
		}
		for _, index := range resolved.Indexes {
			if db.Migrator().HasIndex(model, index.Name) {
				continue
			}
			if err := db.Migrator().CreateIndex(model, index.Name); err != nil {
				return fmt.Errorf("create index %q: %w", index.Name, translateError(err))
			}
		}
	}
	// 所有表和列都准备完成后再创建外键，避免 Schema 输入顺序影响 PostgreSQL/MySQL。
	for _, resolved := range resolvedSchemas {
		db := c.db.WithContext(ctx).Table(resolved.Table)
		for _, reference := range resolved.References {
			constraint := "fk_" + resolved.Table + "_" + resolved.fields[reference.Field].Column
			if db.Migrator().HasConstraint(resolved.Table, constraint) {
				continue
			}
			if db.Dialector.Name() == "sqlite" {
				continue
			}
			if err := addReference(db, resolved, reference, byTable); err != nil {
				return err
			}
		}
	}
	return nil
}

// CheckSchemas 只读校验目标数据库已经具备声明的表、列、索引和外键。
// 它用于运行中配置候选的 Ready 阶段，不执行 migration 或任何 DDL。
func (c *gormClient) CheckSchemas(ctx context.Context, schemas ...Schema) error {
	if c.unavailable() {
		return ErrClientUnavailable
	}
	if err := validateContext(ctx); err != nil {
		return err
	}
	resolvedSchemas := make([]resolvedSchema, 0, len(schemas))
	byTable := make(map[string]resolvedSchema, len(schemas))
	for _, schema := range schemas {
		resolved, err := resolveSchema(schema)
		if err != nil {
			return err
		}
		if _, exists := byTable[resolved.Table]; exists {
			return fmt.Errorf("%w: duplicate schema table %q", ErrInvalidSchema, resolved.Table)
		}
		resolvedSchemas = append(resolvedSchemas, resolved)
		byTable[resolved.Table] = resolved
	}
	if err := validateReferences(byTable); err != nil {
		return err
	}
	for _, resolved := range resolvedSchemas {
		db := c.db.WithContext(ctx).Table(resolved.Table)
		model := resolved.dynamicModel()
		if !db.Migrator().HasTable(resolved.Table) {
			return fmt.Errorf("database schema table %q is missing", resolved.Table)
		}
		for _, field := range resolved.Fields {
			if !db.Migrator().HasColumn(model, field.Name) {
				return fmt.Errorf("database schema column %q.%q is missing", resolved.Table, field.Column)
			}
		}
		for _, index := range resolved.Indexes {
			if !db.Migrator().HasIndex(model, index.Name) {
				return fmt.Errorf("database schema index %q is missing", index.Name)
			}
		}
		for _, reference := range resolved.References {
			constraint := "fk_" + resolved.Table + "_" + resolved.fields[reference.Field].Column
			if !db.Migrator().HasConstraint(resolved.Table, constraint) {
				return fmt.Errorf("database schema constraint %q is missing", constraint)
			}
		}
	}
	return nil
}

func addReference(db *gorm.DB, schema resolvedSchema, reference Reference, schemas map[string]resolvedSchema) error {
	field, exists := schema.fields[reference.Field]
	if !exists {
		return fmt.Errorf("%w: reference field %q is missing", ErrInvalidSchema, reference.Field)
	}
	constraint := "fk_" + schema.Table + "_" + field.Column
	if db.Migrator().HasConstraint(schema.Table, constraint) {
		return nil
	}
	target := schemas[reference.Table]
	targetField := target.fields[reference.ReferenceField]
	statement := "ALTER TABLE " + quote(db, schema.Table) +
		" ADD CONSTRAINT " + quote(db, constraint) +
		" FOREIGN KEY (" + quote(db, field.Column) + ") REFERENCES " +
		quote(db, reference.Table) + " (" + quote(db, targetField.Column) + ")"
	if reference.OnUpdate != "" {
		statement += " ON UPDATE " + string(reference.OnUpdate)
	}
	if reference.OnDelete != "" {
		statement += " ON DELETE " + string(reference.OnDelete)
	}
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("create constraint %q: %w", constraint, translateError(err))
	}
	return nil
}

func createSQLiteTable(db *gorm.DB, schema resolvedSchema, schemas map[string]resolvedSchema) error {
	columns := make([]string, 0, len(schema.Fields)+len(schema.References))
	primaryColumns := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.PrimaryKey {
			primaryColumns = append(primaryColumns, quote(db, field.Column))
		}
	}
	for _, field := range schema.Fields {
		definition := quote(db, field.Column) + " " + sqliteColumnType(field)
		if field.PrimaryKey && len(primaryColumns) == 1 {
			definition += " PRIMARY KEY"
			if field.AutoIncrement {
				definition += " AUTOINCREMENT"
			}
		}
		if !field.Nullable {
			definition += " NOT NULL"
		}
		if field.Default != "" {
			definition += " DEFAULT " + field.Default
		}
		columns = append(columns, definition)
	}
	if len(primaryColumns) > 1 {
		columns = append(columns, "PRIMARY KEY ("+strings.Join(primaryColumns, ", ")+")")
	}
	for _, reference := range schema.References {
		field := schema.fields[reference.Field]
		targetField := schemas[reference.Table].fields[reference.ReferenceField]
		definition := "CONSTRAINT " + quote(db, "fk_"+schema.Table+"_"+field.Column) +
			" FOREIGN KEY (" + quote(db, field.Column) + ") REFERENCES " + quote(db, reference.Table) +
			" (" + quote(db, targetField.Column) + ")"
		if reference.OnUpdate != "" {
			definition += " ON UPDATE " + string(reference.OnUpdate)
		}
		if reference.OnDelete != "" {
			definition += " ON DELETE " + string(reference.OnDelete)
		}
		columns = append(columns, definition)
	}
	statement := "CREATE TABLE " + quote(db, schema.Table) + " (" + strings.Join(columns, ", ") + ")"
	if err := db.Exec(statement).Error; err != nil {
		return fmt.Errorf("create sqlite table %q: %w", schema.Table, translateError(err))
	}
	return nil
}

func sqliteColumnType(field Field) string {
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

func quote(db *gorm.DB, value string) string {
	statement := &gorm.Statement{DB: db}
	return statement.Quote(value)
}
