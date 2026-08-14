# storage

`pkg/storage` 提供两类存储基础能力：

- 文件工具：本地、内存、只读和 base path 文件系统上的读写、复制、监听、MIME、Excel 工作簿和图片辅助。
- 对象存储：通过项目自有 `StorageClient` 契约接入 local、S3、R2、MinIO 等对象存储。

## 边界

- 业务代码依赖 `Storage`、`StorageClient`、`Config`、`Workbook`、`ImageFormat` 等项目类型。
- `afero`、`excelize`、`imaging`、AWS SDK 等第三方类型只留在实现层或适配子包内部。
- 本包不定义业务文件目录、权限模型、上传 API、鉴权策略或全量清空对象存储入口。
- Kernel Storage App 只治理对象存储 Manager；文件工具仍由调用方直接创建、使用和关闭。

## 文件工具示例

```go
fs, err := storage.New(&storage.Config{
    Local: storage.LocalConfig{
        FSType:      storage.FSTypeMemory,
        EnableWatch: false,
    },
})
if err != nil {
    return err
}
defer fs.Close()

if err := fs.WriteFile("hello.txt", []byte("hello"), 0o600); err != nil {
    return err
}
data, err := fs.ReadFile("hello.txt")
```

## Excel 和图片

```go
book := fs.CreateWorkbook()
if err := book.SetCellValue("Sheet1", "A1", "name"); err != nil {
    return err
}
if err := fs.SaveWorkbook(book, "book.xlsx"); err != nil {
    return err
}

if err := fs.ResizeImage("input.png", "small.png", 200, 0, storage.ImageFormatPNG); err != nil {
    return err
}
```

## 对象存储

```go
manager, err := storage.NewManager(ctx, &storage.Config{
    Driver: storage.DriverLocal,
    Local: storage.LocalConfig{BasePath: "./data"},
})
if err != nil {
    return err
}
defer manager.Close()

client := manager.Primary()
if err := client.Put(ctx, "setup/health.txt", []byte("ok"), storage.PutOptions{ContentType: "text/plain"}); err != nil {
    return err
}
```

`StorageManager.Close` 会同时尝试关闭 local 与 object client，并用 `errors.Join` 保留多个关闭错误。

## Kernel 装配

Kernel 组合返回稳定的 `capabilities.Storage`。调用方按 `primary`、`local` 或 `object` route 在回调内借用窄 Client；Client 不含 `Close`，回调结束后不能继续使用：

```go
err := capabilities.Storage.Use(ctx, storageapp.RoutePrimary, func(client storageapp.Client) error {
    return client.Put(ctx, "documents/report.pdf", content, storage.PutOptions{
        ContentType: "application/pdf",
    })
})
```

默认 Driver 为 local，路径是 `.data/storage`。Storage 配置使用 `KernelInstanceSwap`：候选 Manager 构造并通过就绪检查后才切换稳定 Access，失败保留旧实例。S3/MinIO 凭据应通过环境变量提供，不能写入仓库配置。
