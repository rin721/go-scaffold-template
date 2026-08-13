# database

`pkg/database` 为上层业务提供稳定的 Schema、Repository 和事务能力。底层统一使用 GORM，但 GORM 类型、dialector、session、tag 和错误翻译都留在包内；业务实体和公开接口不依赖第三方类型。

## 怎么运行

Kernel 默认使用 pure-Go SQLite，配置无需选择 ORM：

```yaml
database:
  driver: sqlite
  dsn: .data/app.db
```

`internal/kernel/app/database` 在构造代码中调用 `database.NewGORM`，因此运行时配置只能选择 `sqlite`、`postgres` 或 `mysql` Driver，不能切换 GORM/SQLX 等底层技术。SQLite 会自动创建目录和文件，并启用 foreign keys、5 秒 busy timeout 与 WAL；私有 `:memory:` 数据库固定使用一个连接，避免连接池切换后得到不同的内存库。

独立使用时，只有资源所有者负责关闭连接池：

```go
cfg := database.DefaultConfig()
resource, err := database.NewGORM(ctx, &cfg)
if err != nil {
	return err
}
defer resource.Close()

client := resource.Client()
```

PostgreSQL 与 MySQL 应通过环境变量注入真实 DSN，不把凭据写入配置、源码或日志。

## 怎么定义 Schema

业务实体保持普通 Go 结构体，不写 GORM tag。Schema 用项目字段名显式映射数据库列：

```go
type Account struct {
	ID      uint64
	Name    string
	Version uint64
}

schema := database.Schema{
	Table: "accounts",
	Fields: []database.Field{
		{Name: "ID", Column: "id", Type: database.FieldUint64, PrimaryKey: true, AutoIncrement: true},
		{Name: "Name", Column: "name", Type: database.FieldString, Length: 100},
		{Name: "Version", Column: "version", Type: database.FieldUint64},
	},
	Indexes:      []database.Index{{Name: "idx_accounts_name", Fields: []string{"Name"}}},
	VersionField: "Version",
}
```

Schema 支持表、列、主键、nullable、长度、默认值、普通/唯一索引和单列外键关系。Reference 的 Field 与 ReferenceField 都填写各自 Schema 的字段名，不填写数据库列名；目标 Schema 必须与引用方一起传给同一次 Migrate，更新/删除动作使用 `ReferenceCascade`、`ReferenceRestrict`、`ReferenceSetNull` 或 `ReferenceNoAction`。Default 只接受与字段类型匹配的 NULL、布尔、数值、CURRENT_TIMESTAMP 或正确转义的 SQL 单引号字符串，不接受任意 DDL 表达式。`Migrate` 只承诺 additive migration：创建缺失的表、列、索引和约束，不删除、重命名或调整已有列，也不承诺跨数据库 DDL 回滚。

## 怎么使用 Repository

```go
if err := client.Migrate(ctx, schema); err != nil {
	return err
}

accounts, err := database.NewRepository[Account](client, schema)
if err != nil {
	return err
}

created := Account{Name: "Rin"}
if err := accounts.Create(ctx, &created); err != nil {
	return err
}

account, err := accounts.First(ctx, database.Query{Filters: []database.Filter{
	{Field: "ID", Operator: database.OpEqual, Value: created.ID},
}})
```

`BaseRepository[T]` 提供 `Create`、`First`、`Find`、`Count`、`Update` 和 `SoftDelete`。Filter、Order 和 Changes 只接受 Schema 字段名，包内再校验字段和值类型并映射为列名；未知字段、非法值、运算符或排序方向会返回 `ErrInvalidQuery`。`Update` 和 `SoftDelete` 必须有 Filter，且不接受 Order/Page，防止意外全表或含糊修改。

Create 会忽略调用方传入的自增字段值并接收数据库生成值。启用 `VersionField` 后，Create 会把版本统一初始化为 1；Update 必须携带该字段的等值 Filter，包内原子递增版本，零影响行返回 `ErrOptimisticConflict`。启用 `SoftDeleteField` 后，Create 会把该字段统一初始化为 nil，Repository 查询默认排除已经软删除的记录。两个字段均由 Repository 管理，Schema 不能再为其声明 Default、主键或自增语义。

## 怎么使用事务

```go
err := client.WithinTx(ctx, func(ctx context.Context, tx database.Tx) error {
	txAccounts, err := accounts.WithTx(tx)
	if err != nil {
		return err
	}
	return txAccounts.Create(ctx, &Account{Name: "Lin"})
})
```

`Tx` 只是 Repository 重绑定令牌，不提供 Commit、Rollback 或第三方 session。回调返回 `nil` 时提交，返回错误时回滚；回调 panic 时先回滚再继续抛出原 panic。事务对象不得逃逸回调。

## Kernel 租约边界

`Capabilities.Database` 是稳定 Access。`Access.Ping` 在资源租约内提供窄就绪检查，但不暴露连接池对象；上层在 `Use` 回调中取得不含 `Close` 的 Borrowed Client。`Access.WithinTx` 的回调同时取得当前租约内 Client 和 Tx，可在回调中创建 Repository 并重绑定事务。回调返回后，逃逸的 Client、Repository 和 Tx 都会返回 `ErrClientUnavailable`。Stats 和 Close 只由 Kernel 私有 Resource 使用，其动态对象不会通过 Access 暴露。

## 错误语义

调用方使用 `errors.Is` 判断 `ErrNotFound`、`ErrDuplicateKey`、`ErrForeignKeyViolation`、`ErrOptimisticConflict`、`ErrInvalidSchema`、`ErrInvalidQuery`、`ErrUnsafeMutation`、`ErrClientUnavailable`、`ErrNilClientFunc` 和 `ErrNilTransactionFunc`。底层驱动错误只保留 `errors.Is` 可识别性，不提供可展开的原始错误文本，避免 DSN、密码或 Token 通过错误链泄漏。

## 当前非目标

不提供任意 SQL 逃逸口、GORM session 逃逸口、破坏性/版本化迁移、读写分离、分库分表或多租户。确有业务需要时，应扩展项目自有契约并重新确认边界，不能让上层直接依赖 GORM。
