package database

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

type account struct {
	ID        uint64
	Name      string
	Version   uint64
	DeletedAt *time.Time
}

func migrateForTest(ctx context.Context, client Client, schemas ...Schema) error {
	concrete, ok := client.(*gormClient)
	if !ok {
		return ErrClientUnavailable
	}
	return concrete.migrateForTest(ctx, schemas...)
}

func TestSQLiteResourceRepositoryAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "app.db")
	resource, err := NewGORM(context.Background(), &Config{Driver: DriverSQLite, DSN: path})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	t.Cleanup(func() {
		if err := resource.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
	client := resource.Client()
	if runtime.GOOS != "windows" {
		assertMode(t, filepath.Dir(path), 0o700)
		assertMode(t, path, 0o600)
	}

	schema := accountSchema()
	if err := migrateForTest(context.Background(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, err := NewRepository[account](client, schema)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	created := account{ID: 999, Name: "Rin", Version: 99, DeletedAt: pointerTo(time.Now())}
	if err := repository.Create(context.Background(), &created); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.ID == 0 || created.ID == 999 || created.Version != 1 || created.DeletedAt != nil {
		t.Fatalf("Create() managed fields = ID %d, Version %d, DeletedAt %v", created.ID, created.Version, created.DeletedAt)
	}

	query := Query{Filters: []Filter{{Field: "ID", Operator: OpEqual, Value: created.ID}}}
	got, err := repository.First(context.Background(), query)
	if err != nil || got.Name != "Rin" {
		t.Fatalf("First() = %#v, %v", got, err)
	}
	count, err := repository.Count(context.Background(), Query{})
	if err != nil || count != 1 {
		t.Fatalf("Count() = %d, %v", count, err)
	}
	rows, err := repository.Update(context.Background(), Query{Filters: []Filter{
		{Field: "ID", Operator: OpEqual, Value: created.ID},
		{Field: "Version", Operator: OpEqual, Value: uint64(1)},
	}}, Changes{"Name": "Rei"})
	if err != nil || rows != 1 {
		t.Fatalf("Update() = %d, %v", rows, err)
	}
	_, err = repository.Update(context.Background(), Query{Filters: []Filter{
		{Field: "ID", Operator: OpEqual, Value: created.ID},
		{Field: "Version", Operator: OpEqual, Value: uint64(1)},
	}}, Changes{"Name": "stale"})
	if !errors.Is(err, ErrOptimisticConflict) {
		t.Fatalf("stale Update() error = %v", err)
	}
	if _, err := repository.Update(context.Background(), Query{}, Changes{"Name": "unsafe"}); !errors.Is(err, ErrUnsafeMutation) {
		t.Fatalf("unfiltered Update() error = %v", err)
	}
	if _, err := repository.SoftDelete(context.Background(), Query{Filters: []Filter{
		{Field: "ID", Operator: OpEqual, Value: created.ID},
		{Field: "Version", Operator: OpEqual, Value: uint64(2)},
	}}); err != nil {
		t.Fatalf("SoftDelete() error = %v", err)
	}
	if _, err := repository.First(context.Background(), query); !errors.Is(err, ErrNotFound) {
		t.Fatalf("First() after SoftDelete error = %v", err)
	}
	if _, err := repository.Find(context.Background(), Query{Page: &Page{Limit: 0}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Find() invalid page error = %v", err)
	}
	if _, err := repository.Find(context.Background(), Query{Filters: []Filter{{Field: "Unknown", Operator: OpEqual, Value: 1}}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Find() unknown field error = %v", err)
	}
}

type organization struct {
	ID        uint64
	Name      string
	Version   uint64
	DeletedAt *time.Time
}

type membership struct {
	ID             uint64
	OrganizationID uint64
	Name           string
}

func TestSQLiteUniqueAndForeignKeyErrors(t *testing.T) {
	resource, err := NewGORM(context.Background(), &Config{Driver: DriverSQLite, DSN: filepath.Join(t.TempDir(), "app.db")})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	organizationSchema := Schema{
		Table: "organizations",
		Fields: []Field{
			{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "Name", Column: "name", Type: FieldString, Length: 100},
		},
		Indexes: []Index{{Name: "uidx_organizations_name", Fields: []string{"Name"}, Unique: true}},
	}
	membershipSchema := Schema{
		Table: "memberships",
		Fields: []Field{
			{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "OrganizationID", Column: "organization_id", Type: FieldUint64},
			{Name: "Name", Column: "name", Type: FieldString, Length: 100},
		},
		References: []Reference{{Field: "OrganizationID", Table: "organizations", ReferenceField: "ID", OnUpdate: ReferenceCascade, OnDelete: ReferenceRestrict}},
	}
	if err := migrateForTest(context.Background(), client, organizationSchema, membershipSchema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	organizations, _ := NewRepository[organization](client, organizationSchema)
	memberships, _ := NewRepository[membership](client, membershipSchema)
	first := organization{Name: "same"}
	if err := organizations.Create(context.Background(), &first); err != nil {
		t.Fatalf("Create(organization) error = %v", err)
	}
	if err := organizations.Create(context.Background(), &organization{Name: "same"}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("duplicate Create() error = %v", err)
	}
	if err := memberships.Create(context.Background(), &membership{OrganizationID: first.ID + 999, Name: "orphan"}); !errors.Is(err, ErrForeignKeyViolation) {
		t.Fatalf("foreign-key Create() error = %v", err)
	}
}

func TestWithinTxCommitsAndRollsBack(t *testing.T) {
	resource, err := NewGORM(context.Background(), &Config{Driver: DriverSQLite, DSN: ":memory:", Pool: PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	schema := accountSchema()
	if err := migrateForTest(context.Background(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, _ := NewRepository[account](client, schema)
	if err := client.WithinTx(context.Background(), func(ctx context.Context, tx Tx) error {
		txRepository, err := repository.WithTx(tx)
		if err != nil {
			return err
		}
		return txRepository.Create(ctx, &account{Name: "commit", Version: 1})
	}); err != nil {
		t.Fatalf("WithinTx(commit) error = %v", err)
	}
	wantRollback := errors.New("rollback")
	err = client.WithinTx(context.Background(), func(ctx context.Context, tx Tx) error {
		txRepository, _ := repository.WithTx(tx)
		if err := txRepository.Create(ctx, &account{Name: "rollback", Version: 1}); err != nil {
			return err
		}
		return wantRollback
	})
	if !errors.Is(err, wantRollback) {
		t.Fatalf("WithinTx(rollback) error = %v", err)
	}
	count, _ := repository.Count(context.Background(), Query{})
	if count != 1 {
		t.Fatalf("Count() after rollback = %d", count)
	}
}

func TestResourceCloseInvalidatesClientAndIsIdempotent(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:", Pool: PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	client := resource.Client()
	if err := resource.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := resource.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}
	if err := migrateForTest(t.Context(), client, accountSchema()); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("closed Client error = %v, want ErrClientUnavailable", err)
	}
	if err := resource.Ping(t.Context()); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("closed Resource Ping error = %v, want ErrClientUnavailable", err)
	}
}

func TestPrivateSQLiteMemoryUsesSingleConnection(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:"})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	if got := resource.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1", got)
	}
}

func TestDirectTransactionInvalidatesEscapedTx(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:", Pool: PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	if err := migrateForTest(t.Context(), client, accountSchema()); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, _ := NewRepository[account](client, accountSchema())
	var escaped Tx
	if err := client.WithinTx(t.Context(), func(_ context.Context, tx Tx) error {
		escaped = tx
		return nil
	}); err != nil {
		t.Fatalf("WithinTx() error = %v", err)
	}
	txRepository, _ := repository.WithTx(escaped)
	if _, err := txRepository.Count(t.Context(), Query{}); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("escaped direct Tx error = %v, want ErrClientUnavailable", err)
	}
}

func TestTransactionPanicRollsBackAndPreservesPanic(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:"})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	schema := accountSchema()
	if err := migrateForTest(t.Context(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, err := NewRepository[account](client, schema)
	if err != nil {
		t.Fatalf("NewRepository() error = %v", err)
	}
	panicValue := &struct{ name string }{name: "domain panic"}
	func() {
		defer func() {
			if recovered := recover(); recovered != panicValue {
				t.Fatalf("recovered panic = %#v, want original value", recovered)
			}
		}()
		_ = client.WithinTx(t.Context(), func(ctx context.Context, tx Tx) error {
			txRepository, bindErr := repository.WithTx(tx)
			if bindErr != nil {
				t.Fatalf("WithTx() error = %v", bindErr)
			}
			if createErr := txRepository.Create(ctx, &account{Name: "panic"}); createErr != nil {
				t.Fatalf("Create() error = %v", createErr)
			}
			panic(panicValue)
		})
	}()
	count, err := repository.Count(t.Context(), Query{})
	if err != nil || count != 0 {
		t.Fatalf("Count() after panic = %d, %v", count, err)
	}
}

func TestTransactionOperationCannotBypassRootCancellation(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:"})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	schema := accountSchema()
	if err := migrateForTest(t.Context(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, _ := NewRepository[account](client, schema)
	txCtx, cancel := context.WithCancel(t.Context())
	wantErr := errors.New("transaction cancelled")
	err = client.WithinTx(txCtx, func(_ context.Context, tx Tx) error {
		cancel()
		txRepository, bindErr := repository.WithTx(tx)
		if bindErr != nil {
			return bindErr
		}
		if _, countErr := txRepository.Count(context.Background(), Query{}); !errors.Is(countErr, context.Canceled) {
			t.Fatalf("Count() error = %v, want context.Canceled", countErr)
		}
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("WithinTx() error = %v, want callback error", err)
	}
}

func TestBorrowInvalidatesEscapedRepositoryAndTransaction(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:", Pool: PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	schema := accountSchema()
	if err := migrateForTest(t.Context(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	var escapedRepository *BaseRepository[account]
	var escapedTx Tx
	if err := Borrow(t.Context(), client, func(borrowed Client) error {
		repository, err := NewRepository[account](borrowed, schema)
		if err != nil {
			return err
		}
		escapedRepository = repository
		if err := borrowed.WithinTx(t.Context(), func(_ context.Context, tx Tx) error {
			escapedTx = tx
			return nil
		}); err != nil {
			return err
		}
		txRepository, err := repository.WithTx(escapedTx)
		if err != nil {
			return err
		}
		if _, err := txRepository.Count(t.Context(), Query{}); !errors.Is(err, ErrClientUnavailable) {
			t.Fatalf("Tx remained usable after transaction callback: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("Borrow() error = %v", err)
	}
	if _, err := escapedRepository.Count(t.Context(), Query{}); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("escaped Repository error = %v, want ErrClientUnavailable", err)
	}
	if err := escapedRepository.session.(*borrowedClient).WithinTx(t.Context(), nil); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("escaped Client nil callback error = %v, want ErrClientUnavailable", err)
	}
	if _, err := escapedRepository.WithTx(escapedTx); err != nil {
		t.Fatalf("WithTx(escaped) constructor error = %v", err)
	}
	txRepository, _ := escapedRepository.WithTx(escapedTx)
	if _, err := txRepository.Count(t.Context(), Query{}); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("escaped Tx Repository error = %v, want ErrClientUnavailable", err)
	}
}

func TestConfigAndQueryValidation(t *testing.T) {
	if err := ValidateConfig(&Config{Driver: Driver("oracle"), DSN: "secret"}); !errors.Is(err, ErrInvalidDriver) {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
	secret := "postgres://user:top-secret@example.invalid/app"
	resource, err := NewGORM(context.Background(), &Config{Driver: DriverPostgres, DSN: secret, PingTimeout: time.Millisecond})
	if err != nil {
		t.Fatalf("NewGORM() should not ping: %v", err)
	}
	defer resource.Close()
	err = resource.Ping(context.Background())
	if err == nil {
		t.Fatal("Ping() returned nil error")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "top-secret") {
		t.Fatalf("error leaks DSN: %v", err)
	}
	assertErrorTreeRedacted(t, err, secret, "top-secret")
	if _, err := resolveSchema(Schema{Table: "accounts;drop", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}}}); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("resolveSchema() error = %v", err)
	}
	if _, err := resolveSchema(Schema{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}, {Name: "Name", Column: "name", Type: FieldString, Default: "x'); DROP TABLE accounts; --"}}}); !errors.Is(err, ErrInvalidSchema) {
		t.Fatalf("resolveSchema(unsafe default) error = %v", err)
	}
	var nilClient *gormClient
	if _, err := NewRepository[account](nilClient, accountSchema()); err == nil {
		t.Fatal("NewRepository(typed nil) error = nil")
	}
	if err := Borrow(t.Context(), nilClient, func(Client) error { return nil }); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("Borrow(typed nil) error = %v, want ErrClientUnavailable", err)
	}
	if err := Borrow(t.Context(), &gormClient{}, nil); !errors.Is(err, ErrNilClientFunc) {
		t.Fatalf("Borrow(nil func) error = %v, want ErrNilClientFunc", err)
	}
	closedClient := &gormClient{}
	closedClient.closed.Store(true)
	if err := closedClient.WithinTx(t.Context(), nil); !errors.Is(err, ErrClientUnavailable) {
		t.Fatalf("closed client precedence error = %v, want ErrClientUnavailable", err)
	}
	invalidSchemas := []Schema{
		{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, Nullable: true}}},
		{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}, {Name: "Enabled", Column: "enabled", Type: FieldBool, Default: "1"}}},
		{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}, {Name: "Version", Column: "version", Type: FieldUint64, Nullable: true}}, VersionField: "Version"},
		{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}, {Name: "Version", Column: "version", Type: FieldUint64, Default: "1"}}, VersionField: "Version"},
		{Table: "accounts", Fields: []Field{{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true}, {Name: "DeletedAt", Column: "deleted_at", Type: FieldTime, Nullable: true, Default: "CURRENT_TIMESTAMP"}}, SoftDeleteField: "DeletedAt"},
	}
	for _, schema := range invalidSchemas {
		if _, err := resolveSchema(schema); !errors.Is(err, ErrInvalidSchema) {
			t.Fatalf("resolveSchema(%+v) error = %v, want ErrInvalidSchema", schema, err)
		}
	}
}

func TestResourceCloseCachesFirstTerminalResult(t *testing.T) {
	cause := errors.New("close failed")
	attempts := 0
	resource := &gormResource{client: &gormClient{}, close: func() error {
		attempts++
		return cause
	}}
	first := resource.Close()
	second := resource.Close()
	if !errors.Is(first, cause) || !errors.Is(second, cause) {
		t.Fatalf("Close() errors = %v / %v, want cause", first, second)
	}
	if attempts != 1 {
		t.Fatalf("Close() attempts = %d, want 1", attempts)
	}
}

func pointerTo[T any](value T) *T { return &value }

func assertErrorTreeRedacted(t *testing.T, err error, forbidden ...string) {
	t.Helper()
	visited := map[error]struct{}{}
	var inspect func(error)
	inspect = func(current error) {
		if current == nil {
			return
		}
		if _, exists := visited[current]; exists {
			return
		}
		visited[current] = struct{}{}
		for _, value := range forbidden {
			if strings.Contains(current.Error(), value) {
				t.Fatalf("error tree leaks %q: %v", value, current)
			}
		}
		switch value := current.(type) {
		case interface{ Unwrap() []error }:
			for _, nested := range value.Unwrap() {
				inspect(nested)
			}
		case interface{ Unwrap() error }:
			inspect(value.Unwrap())
		}
	}
	inspect(err)
}

func TestRepositoryRejectsInvalidValuesBeforeDriver(t *testing.T) {
	resource, err := NewGORM(t.Context(), &Config{Driver: DriverSQLite, DSN: ":memory:", Pool: PoolConfig{MaxOpenConns: 1, MaxIdleConns: 1}})
	if err != nil {
		t.Fatalf("NewGORM() error = %v", err)
	}
	defer resource.Close()
	client := resource.Client()
	schema := accountSchema()
	if err := migrateForTest(t.Context(), client, schema); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repository, _ := NewRepository[account](client, schema)
	tests := []struct {
		name  string
		query Query
	}{
		{name: "wrong type", query: Query{Filters: []Filter{{Field: "ID", Operator: OpEqual, Value: "1"}}}},
		{name: "empty in", query: Query{Filters: []Filter{{Field: "ID", Operator: OpIn, Value: []uint64{}}}}},
		{name: "like number", query: Query{Filters: []Filter{{Field: "ID", Operator: OpLike, Value: "1%"}}}},
		{name: "null non-nullable", query: Query{Filters: []Filter{{Field: "Name", Operator: OpIsNull}}}},
		{name: "null with value", query: Query{Filters: []Filter{{Field: "DeletedAt", Operator: OpIsNull, Value: time.Now()}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := repository.Find(t.Context(), tt.query); !errors.Is(err, ErrInvalidQuery) {
				t.Fatalf("Find() error = %v, want ErrInvalidQuery", err)
			}
		})
	}
	if _, err := repository.Update(t.Context(), Query{Filters: []Filter{{Field: "ID", Operator: OpEqual, Value: uint64(1)}}}, Changes{"Name": strings.Repeat("x", 101)}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("Update(long value) error = %v, want ErrInvalidQuery", err)
	}
	if _, err := repository.First(t.Context(), Query{Page: &Page{Limit: 1}}); !errors.Is(err, ErrInvalidQuery) {
		t.Fatalf("First(page) error = %v, want ErrInvalidQuery", err)
	}
}

func TestConfiguredServerDrivers(t *testing.T) {
	tests := []struct {
		name   string
		driver Driver
		env    string
	}{
		{name: "postgres", driver: DriverPostgres, env: "TEST_DATABASE_POSTGRES_DSN"},
		{name: "mysql", driver: DriverMySQL, env: "TEST_DATABASE_MYSQL_DSN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skipf("%s is not configured", tt.env)
			}
			resource, err := NewGORM(t.Context(), &Config{Driver: tt.driver, DSN: dsn})
			if err != nil {
				t.Fatalf("NewGORM(%s) error = %v", tt.driver, err)
			}
			defer resource.Close()
			if err := resource.Ping(t.Context()); err != nil {
				t.Fatalf("Ping(%s) error = %v", tt.driver, err)
			}
			prefix := "dc_" + tt.name[:1] + "_" + strconv.FormatInt(time.Now().UnixNano(), 36)
			runServerRepositoryContract(t, resource, prefix)
		})
	}
}

func runServerRepositoryContract(t *testing.T, resource Resource, prefix string) {
	t.Helper()
	client := resource.Client()
	organizationsTable := prefix + "_organizations"
	membershipsTable := prefix + "_memberships"
	organizationSchema := Schema{
		Table: organizationsTable,
		Fields: []Field{
			{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "Name", Column: "name", Type: FieldString, Length: 100},
			{Name: "Version", Column: "version", Type: FieldUint64},
			{Name: "DeletedAt", Column: "deleted_at", Type: FieldTime, Nullable: true},
		},
		Indexes:      []Index{{Name: prefix + "_uidx_organizations_name", Fields: []string{"Name"}, Unique: true}},
		VersionField: "Version", SoftDeleteField: "DeletedAt",
	}
	membershipSchema := Schema{
		Table: membershipsTable,
		Fields: []Field{
			{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "OrganizationID", Column: "organization_id", Type: FieldUint64},
			{Name: "Name", Column: "name", Type: FieldString, Length: 100},
		},
		References: []Reference{{Field: "OrganizationID", Table: organizationsTable, ReferenceField: "ID", OnUpdate: ReferenceCascade, OnDelete: ReferenceRestrict}},
	}
	if err := migrateForTest(t.Context(), client, organizationSchema, membershipSchema); err != nil {
		t.Fatalf("Migrate(server contract) error = %v", err)
	}
	organizations, err := NewRepository[organization](client, organizationSchema)
	if err != nil {
		t.Fatalf("NewRepository(organization) error = %v", err)
	}
	memberships, err := NewRepository[membership](client, membershipSchema)
	if err != nil {
		t.Fatalf("NewRepository(membership) error = %v", err)
	}
	created := organization{Name: "Rin", Version: 1}
	if err := organizations.Create(t.Context(), &created); err != nil {
		t.Fatalf("Create(server organization) error = %v", err)
	}
	if err := organizations.Create(t.Context(), &organization{Name: "Rin", Version: 1}); !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("duplicate server Create() error = %v", err)
	}
	if err := memberships.Create(t.Context(), &membership{OrganizationID: created.ID + 999, Name: "orphan"}); !errors.Is(err, ErrForeignKeyViolation) {
		t.Fatalf("foreign-key server Create() error = %v", err)
	}
	query := Query{Filters: []Filter{
		{Field: "ID", Operator: OpEqual, Value: created.ID},
		{Field: "Version", Operator: OpEqual, Value: uint64(1)},
	}}
	if rows, err := organizations.Update(t.Context(), query, Changes{"Name": "Rei"}); err != nil || rows != 1 {
		t.Fatalf("Update(server organization) = %d, %v", rows, err)
	}
	if _, err := organizations.Update(t.Context(), query, Changes{"Name": "stale"}); !errors.Is(err, ErrOptimisticConflict) {
		t.Fatalf("stale server Update() error = %v", err)
	}
	if err := client.WithinTx(t.Context(), func(ctx context.Context, tx Tx) error {
		txRepository, err := memberships.WithTx(tx)
		if err != nil {
			return err
		}
		return txRepository.Create(ctx, &membership{OrganizationID: created.ID, Name: "member"})
	}); err != nil {
		t.Fatalf("WithinTx(server contract) error = %v", err)
	}
}

func accountSchema() Schema {
	return Schema{
		Table: "accounts",
		Fields: []Field{
			{Name: "ID", Column: "id", Type: FieldUint64, PrimaryKey: true, AutoIncrement: true},
			{Name: "Name", Column: "name", Type: FieldString, Length: 100},
			{Name: "Version", Column: "version", Type: FieldUint64},
			{Name: "DeletedAt", Column: "deleted_at", Type: FieldTime, Nullable: true},
		},
		Indexes:      []Index{{Name: "idx_accounts_name", Fields: []string{"Name"}}},
		VersionField: "Version", SoftDeleteField: "DeletedAt",
	}
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("mode(%q) = %o, want %o", path, got, want)
	}
}
