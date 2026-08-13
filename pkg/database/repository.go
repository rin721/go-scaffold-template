package database

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// BaseRepository 提供受 Schema 约束的通用 CRUD 能力。
type BaseRepository[T any] struct {
	schema  resolvedSchema
	session sessionProvider
}

// NewRepository 创建绑定实体和 Schema 的仓储。
func NewRepository[T any](client Client, schema Schema) (*BaseRepository[T], error) {
	provider, ok := client.(sessionProvider)
	if !ok || isNilProvider(provider) {
		return nil, fmt.Errorf("%w: client does not support repositories", ErrClientUnavailable)
	}
	resolved, err := resolveSchema(schema)
	if err != nil {
		return nil, err
	}
	if err := validateEntityType[T](resolved); err != nil {
		return nil, err
	}
	return &BaseRepository[T]{schema: resolved, session: provider}, nil
}

// WithTx 返回绑定到指定事务的仓储；原仓储保持不变。
func (r *BaseRepository[T]) WithTx(tx Tx) (*BaseRepository[T], error) {
	provider, ok := tx.(sessionProvider)
	if !ok || isNilProvider(provider) {
		return nil, fmt.Errorf("%w: transaction does not support repositories", ErrClientUnavailable)
	}
	return &BaseRepository[T]{schema: r.schema, session: provider}, nil
}

// Create 创建单个实体。
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	if err := validateContext(ctx); err != nil {
		return err
	}
	if entity == nil {
		return fmt.Errorf("%w: entity is nil", ErrInvalidQuery)
	}
	model, err := r.modelFromEntity(entity)
	if err != nil {
		return err
	}
	db, err := r.db(ctx)
	if err != nil {
		return err
	}
	if err := translateError(db.Create(model).Error); err != nil {
		return err
	}
	return r.copyModelToEntity(model, entity)
}

// First 查询第一条匹配记录；未命中返回 ErrNotFound。
func (r *BaseRepository[T]) First(ctx context.Context, query Query) (T, error) {
	var entity T
	if query.Page != nil {
		return entity, fmt.Errorf("%w: First does not accept page", ErrInvalidQuery)
	}
	db, err := r.query(ctx, query, false)
	if err != nil {
		return entity, err
	}
	model := r.schema.dynamicModel()
	if err := db.First(model).Error; err != nil {
		return entity, translateError(err)
	}
	if err := r.copyModelToEntity(model, &entity); err != nil {
		return entity, err
	}
	return entity, nil
}

// Find 查询匹配记录。
func (r *BaseRepository[T]) Find(ctx context.Context, query Query) ([]T, error) {
	db, err := r.query(ctx, query, true)
	if err != nil {
		return nil, err
	}
	modelType := reflect.TypeOf(r.schema.dynamicModel()).Elem()
	models := reflect.New(reflect.SliceOf(modelType))
	if err := db.Find(models.Interface()).Error; err != nil {
		return nil, translateError(err)
	}
	entities := make([]T, models.Elem().Len())
	for index := range entities {
		if err := r.copyModelToEntity(models.Elem().Index(index).Addr().Interface(), &entities[index]); err != nil {
			return nil, err
		}
	}
	return entities, nil
}

// Count 返回匹配记录数；忽略 Page 和 Order。
func (r *BaseRepository[T]) Count(ctx context.Context, query Query) (int64, error) {
	db, err := r.query(ctx, Query{Filters: query.Filters}, false)
	if err != nil {
		return 0, err
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return 0, translateError(err)
	}
	return count, nil
}

// Update 更新匹配记录。空筛选会返回 ErrUnsafeMutation。
//
// Schema 启用 VersionField 时，query 必须包含该字段等值条件；更新会原子递增版本。
func (r *BaseRepository[T]) Update(ctx context.Context, query Query, changes Changes) (int64, error) {
	if err := validateContext(ctx); err != nil {
		return 0, err
	}
	if len(query.Orders) > 0 || query.Page != nil {
		return 0, fmt.Errorf("%w: mutation query does not accept order or page", ErrInvalidQuery)
	}
	if len(query.Filters) == 0 {
		return 0, ErrUnsafeMutation
	}
	if len(changes) == 0 {
		return 0, fmt.Errorf("%w: changes are empty", ErrInvalidQuery)
	}
	updates := make(map[string]any, len(changes)+1)
	for name, value := range changes {
		field, exists := r.schema.fields[name]
		if !exists {
			return 0, fmt.Errorf("%w: unknown field %q", ErrInvalidQuery, name)
		}
		if field.PrimaryKey || field.AutoIncrement || name == r.schema.VersionField || name == r.schema.SoftDeleteField {
			return 0, fmt.Errorf("%w: field %q is managed by the repository", ErrInvalidQuery, name)
		}
		if err := validateFieldValue(field, value); err != nil {
			return 0, err
		}
		updates[field.Column] = value
	}
	if r.schema.VersionField != "" {
		if _, err := r.requireVersionFilter(query); err != nil {
			return 0, err
		}
	}
	db, err := r.query(ctx, Query{Filters: query.Filters}, false)
	if err != nil {
		return 0, err
	}
	if r.schema.VersionField != "" {
		versionColumn, _ := r.schema.column(r.schema.VersionField)
		updates[versionColumn] = gorm.Expr(quote(db, versionColumn) + " + 1")
	}
	result := db.Updates(updates)
	if result.Error != nil {
		return 0, translateError(result.Error)
	}
	if r.schema.VersionField != "" && result.RowsAffected == 0 {
		return 0, ErrOptimisticConflict
	}
	return result.RowsAffected, nil
}

// SoftDelete 标记匹配记录已删除。空筛选会返回 ErrUnsafeMutation。
func (r *BaseRepository[T]) SoftDelete(ctx context.Context, query Query) (int64, error) {
	if err := validateContext(ctx); err != nil {
		return 0, err
	}
	if len(query.Orders) > 0 || query.Page != nil {
		return 0, fmt.Errorf("%w: mutation query does not accept order or page", ErrInvalidQuery)
	}
	if len(query.Filters) == 0 {
		return 0, ErrUnsafeMutation
	}
	if r.schema.SoftDeleteField == "" {
		return 0, fmt.Errorf("%w: soft delete field is not configured", ErrInvalidSchema)
	}
	column, _ := r.schema.column(r.schema.SoftDeleteField)
	updates := map[string]any{column: gorm.Expr("CURRENT_TIMESTAMP")}
	if r.schema.VersionField != "" {
		if _, err := r.requireVersionFilter(query); err != nil {
			return 0, err
		}
	}
	db, err := r.query(ctx, Query{Filters: query.Filters}, false)
	if err != nil {
		return 0, err
	}
	if r.schema.VersionField != "" {
		versionColumn, _ := r.schema.column(r.schema.VersionField)
		updates[versionColumn] = gorm.Expr(quote(db, versionColumn) + " + 1")
	}
	result := db.Updates(updates)
	if result.Error != nil {
		return 0, translateError(result.Error)
	}
	if r.schema.VersionField != "" && result.RowsAffected == 0 {
		return 0, ErrOptimisticConflict
	}
	return result.RowsAffected, nil
}

func (r *BaseRepository[T]) db(ctx context.Context) (*gorm.DB, error) {
	session, err := r.session.databaseSession(ctx)
	if err != nil {
		return nil, err
	}
	db := session.(*gorm.DB).Table(r.schema.Table)
	if r.schema.SoftDeleteField != "" {
		column, _ := r.schema.column(r.schema.SoftDeleteField)
		db = db.Where(quote(db, column) + " IS NULL")
	}
	return db, nil
}

func (r *BaseRepository[T]) query(ctx context.Context, query Query, includePage bool) (*gorm.DB, error) {
	if err := validateContext(ctx); err != nil {
		return nil, err
	}
	db, err := r.db(ctx)
	if err != nil {
		return nil, err
	}
	for _, filter := range query.Filters {
		field, exists := r.schema.fields[filter.Field]
		if !exists {
			return nil, fmt.Errorf("%w: unknown field %q", ErrInvalidQuery, filter.Field)
		}
		if err := validateFilter(field, filter); err != nil {
			return nil, err
		}
		condition, needsValue, err := filterCondition(quote(db, field.Column), filter.Operator)
		if err != nil {
			return nil, err
		}
		if needsValue {
			db = db.Where(condition, filter.Value)
		} else {
			db = db.Where(condition)
		}
	}
	for _, order := range query.Orders {
		field, exists := r.schema.fields[order.Field]
		if !exists {
			return nil, fmt.Errorf("%w: unknown field %q", ErrInvalidQuery, order.Field)
		}
		if !orderedField(field.Type) {
			return nil, fmt.Errorf("%w: field %q does not support ordering", ErrInvalidQuery, order.Field)
		}
		direction := strings.ToUpper(string(order.Direction))
		if direction != "ASC" && direction != "DESC" {
			return nil, fmt.Errorf("%w: invalid order direction %q", ErrInvalidQuery, order.Direction)
		}
		db = db.Order(quote(db, field.Column) + " " + direction)
	}
	if includePage && query.Page != nil {
		if query.Page.Offset < 0 || query.Page.Limit <= 0 {
			return nil, fmt.Errorf("%w: invalid page", ErrInvalidQuery)
		}
		db = db.Offset(query.Page.Offset).Limit(query.Page.Limit)
	}
	return db, nil
}

func (r *BaseRepository[T]) requireVersionFilter(query Query) (string, error) {
	column, _ := r.schema.column(r.schema.VersionField)
	for _, filter := range query.Filters {
		if filter.Field == r.schema.VersionField && filter.Operator == OpEqual {
			return column, nil
		}
	}
	return "", fmt.Errorf("%w: version equality filter is required", ErrInvalidQuery)
}

func (r *BaseRepository[T]) modelFromEntity(entity *T) (any, error) {
	model := r.schema.dynamicModel()
	entityValue := reflect.ValueOf(entity).Elem()
	modelValue := reflect.ValueOf(model).Elem()
	for _, field := range r.schema.Fields {
		name := exportedName(field.Name)
		source := entityValue.FieldByName(name)
		target := modelValue.FieldByName(name)
		if !source.IsValid() || !target.IsValid() || !source.Type().AssignableTo(target.Type()) {
			return nil, fmt.Errorf("%w: entity field %q is missing or has incompatible type", ErrInvalidSchema, name)
		}
		if field.AutoIncrement {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		if field.Name == r.schema.VersionField {
			target.Set(initialVersionValue(target.Type()))
			continue
		}
		if field.Name == r.schema.SoftDeleteField {
			target.Set(reflect.Zero(target.Type()))
			continue
		}
		if err := validateFieldValue(field, source.Interface()); err != nil {
			return nil, err
		}
		target.Set(source)
	}
	return model, nil
}

func initialVersionValue(valueType reflect.Type) reflect.Value {
	value := reflect.New(valueType).Elem()
	if valueType.Kind() >= reflect.Int && valueType.Kind() <= reflect.Int64 {
		value.SetInt(1)
	} else {
		value.SetUint(1)
	}
	return value
}

func validateEntityType[T any](schema resolvedSchema) error {
	typeOf := reflect.TypeOf((*T)(nil)).Elem()
	if typeOf.Kind() != reflect.Struct {
		return fmt.Errorf("%w: repository entity must be a struct", ErrInvalidSchema)
	}
	for _, field := range schema.Fields {
		entityField, exists := typeOf.FieldByName(field.Name)
		if !exists || entityField.PkgPath != "" || entityField.Type != fieldReflectType(field) {
			return fmt.Errorf("%w: entity field %q is missing or has incompatible type", ErrInvalidSchema, field.Name)
		}
	}
	return nil
}

func validateFilter(field Field, filter Filter) error {
	switch filter.Operator {
	case OpIsNull, OpIsNotNull:
		if !field.Nullable {
			return fmt.Errorf("%w: null operator requires nullable field %q", ErrInvalidQuery, field.Name)
		}
		if !nilValue(filter.Value) {
			return fmt.Errorf("%w: null operator does not accept a value for field %q", ErrInvalidQuery, field.Name)
		}
		return nil
	case OpIn, OpNotIn:
		value := reflect.ValueOf(filter.Value)
		if !value.IsValid() || (value.Kind() != reflect.Slice && value.Kind() != reflect.Array) || value.Len() == 0 {
			return fmt.Errorf("%w: operator %q requires a non-empty collection", ErrInvalidQuery, filter.Operator)
		}
		for index := 0; index < value.Len(); index++ {
			if err := validateFieldValue(field, value.Index(index).Interface()); err != nil {
				return err
			}
		}
		return nil
	case OpLike:
		if field.Type != FieldString {
			return fmt.Errorf("%w: LIKE requires string field %q", ErrInvalidQuery, field.Name)
		}
		if _, ok := filter.Value.(string); !ok {
			return fmt.Errorf("%w: LIKE requires string value for field %q", ErrInvalidQuery, field.Name)
		}
		return nil
	case OpLessThan, OpLessOrEqual, OpGreaterThan, OpGreaterOrEqual:
		if !orderedField(field.Type) {
			return fmt.Errorf("%w: operator %q is not supported for field %q", ErrInvalidQuery, filter.Operator, field.Name)
		}
	}
	if nilValue(filter.Value) {
		return fmt.Errorf("%w: nil filter value requires null operator for field %q", ErrInvalidQuery, field.Name)
	}
	return validateFieldValue(field, filter.Value)
}

func validateFieldValue(field Field, value any) error {
	if nilValue(value) {
		if field.Nullable {
			return nil
		}
		return fmt.Errorf("%w: field %q does not accept nil", ErrInvalidQuery, field.Name)
	}
	typeOf := reflect.TypeOf(value)
	expected := fieldReflectType(field)
	if typeOf != expected {
		// Nullable 字段的查询和更新允许直接传底层值，不强迫调用方临时构造指针。
		if !(field.Nullable && expected.Kind() == reflect.Pointer && typeOf == expected.Elem()) {
			return fmt.Errorf("%w: field %q value has type %s, want %s", ErrInvalidQuery, field.Name, typeOf, expected)
		}
	}
	if field.Type == FieldString && field.Length > 0 {
		stringValue := value
		if typeOf.Kind() == reflect.Pointer {
			if reflect.ValueOf(value).IsNil() {
				return nil
			}
			stringValue = reflect.ValueOf(value).Elem().Interface()
		}
		if text, ok := stringValue.(string); ok && len([]rune(text)) > field.Length {
			return fmt.Errorf("%w: field %q exceeds length %d", ErrInvalidQuery, field.Name, field.Length)
		}
	}
	return nil
}

func nilValue(value any) bool {
	return isNilValue(value)
}

func isNilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func orderedField(fieldType FieldType) bool {
	switch fieldType {
	case FieldInt, FieldInt64, FieldUint, FieldUint64, FieldFloat64, FieldString, FieldTime:
		return true
	default:
		return false
	}
}

func isNilProvider(provider sessionProvider) bool {
	if provider == nil {
		return true
	}
	value := reflect.ValueOf(provider)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func (r *BaseRepository[T]) copyModelToEntity(model any, entity *T) error {
	modelValue := reflect.ValueOf(model)
	if modelValue.Kind() == reflect.Pointer {
		modelValue = modelValue.Elem()
	}
	entityValue := reflect.ValueOf(entity).Elem()
	for _, field := range r.schema.Fields {
		name := exportedName(field.Name)
		source := modelValue.FieldByName(name)
		target := entityValue.FieldByName(name)
		if !source.IsValid() || !target.IsValid() || !target.CanSet() || !source.Type().AssignableTo(target.Type()) {
			return fmt.Errorf("%w: entity field %q is missing or has incompatible type", ErrInvalidSchema, name)
		}
		target.Set(source)
	}
	return nil
}

func filterCondition(column string, operator Operator) (string, bool, error) {
	switch operator {
	case OpEqual:
		return column + " = ?", true, nil
	case OpNotEqual:
		return column + " <> ?", true, nil
	case OpLessThan:
		return column + " < ?", true, nil
	case OpLessOrEqual:
		return column + " <= ?", true, nil
	case OpGreaterThan:
		return column + " > ?", true, nil
	case OpGreaterOrEqual:
		return column + " >= ?", true, nil
	case OpIn:
		return column + " IN ?", true, nil
	case OpNotIn:
		return column + " NOT IN ?", true, nil
	case OpLike:
		return column + " LIKE ?", true, nil
	case OpIsNull:
		return column + " IS NULL", false, nil
	case OpIsNotNull:
		return column + " IS NOT NULL", false, nil
	default:
		return "", false, fmt.Errorf("%w: invalid filter operator %q", ErrInvalidQuery, operator)
	}
}
