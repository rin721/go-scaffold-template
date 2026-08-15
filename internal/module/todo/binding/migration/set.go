// Package migration 绑定 Todo module 独占的三驱动 versioned SQL。
package migration

import (
	"embed"

	"github.com/rin721/go-scaffold-template/pkg/database"
	dbmigrate "github.com/rin721/go-scaffold-template/pkg/database/migrate"
)

const (
	// CurrentVersion 是当前 Todo 二进制唯一兼容的 schema 版本。
	CurrentVersion uint = 3
	// TableName 是 Todo migration set 的版本表。
	TableName = "schema_migrations"
)

//go:embed sqlite/*.sql postgres/*.sql mysql/*.sql
var sqlFiles embed.FS

// Set 返回 Todo-owned migration authority；checksum manifest 会阻止已发布 SQL 被静默改写。
func Set() dbmigrate.Set {
	return dbmigrate.Set{
		Name: "todo", FS: sqlFiles, CurrentVersion: CurrentVersion, MigrationsTable: TableName,
		DriverPaths: map[database.Driver]string{
			database.DriverSQLite: "sqlite", database.DriverPostgres: "postgres", database.DriverMySQL: "mysql",
		},
		SHA256ByFile: map[string]string{
			"sqlite/000001_create_todos.up.sql":              "4d1fb6b5e9fcfb9c70c029e1a15eca6aa3bf637328812088ab134cc4826eaa1a",
			"sqlite/000001_create_todos.down.sql":            "d90654d38441d45165907144d4b536f44f904dbf5f9a547b33288dd33343f9be",
			"sqlite/000002_add_owner_subject.up.sql":         "9ad9993b564caf40862c9ca01b208a32180fe59c577284a07b0cc63614cb5a03",
			"sqlite/000002_add_owner_subject.down.sql":       "06e89be6421d7d97036cf5b7b6b5a4b7597cf6381da0f086b9cf683baf465289",
			"sqlite/000003_require_owner_subject.up.sql":     "683cbc8c84499e4e94a414c3393680579e110a01ec59a9b0cbcbe63714b52d67",
			"sqlite/000003_require_owner_subject.down.sql":   "8d88c085448597b73c96fd828b1ae05b43df1fa9674084aaf8764cc67d39e623",
			"postgres/000001_create_todos.up.sql":            "4420e38bee16f3e1b36f4d702f6d4a8fce86687b5168227c90b9c71192cd49f1",
			"postgres/000001_create_todos.down.sql":          "d90654d38441d45165907144d4b536f44f904dbf5f9a547b33288dd33343f9be",
			"postgres/000002_add_owner_subject.up.sql":       "2122f6ab34bb3cdfe868e3138975371a3b343cea2d01f9e0109e3959ac5b06cb",
			"postgres/000002_add_owner_subject.down.sql":     "0c9ac759852a7ae73ef79899d5391338d0fdd8f62d59effae6fa9c3a7d36af68",
			"postgres/000003_require_owner_subject.up.sql":   "cacb234b913dcb99406a4c688ed142222c2847dff069bc81b9b3a320f08b0384",
			"postgres/000003_require_owner_subject.down.sql": "ed5317d04a76aec46e3adc5b881344b7ce1dd80397e15fae2e09166b89efbe93",
			"mysql/000001_create_todos.up.sql":               "c72700d1dee08ae385eb8e77df301c7ddecdabf0e2817de784a958b8d523ac56",
			"mysql/000001_create_todos.down.sql":             "d90654d38441d45165907144d4b536f44f904dbf5f9a547b33288dd33343f9be",
			"mysql/000002_add_owner_subject.up.sql":          "3d7d9d3c8930c3e77844c3c80df72596e3d22f9dfae6960542e2ae2db0edbb48",
			"mysql/000002_add_owner_subject.down.sql":        "06e89be6421d7d97036cf5b7b6b5a4b7597cf6381da0f086b9cf683baf465289",
			"mysql/000003_require_owner_subject.up.sql":      "e68e4605a4b3092593b3b6f0db04b757a4d40f884ad9034e909b5af72a3daf91",
			"mysql/000003_require_owner_subject.down.sql":    "da9b1b80fda0ec6ec574073df18a3b93915c3ac20551d6ac3eb8d6d610351023",
		},
	}
}
