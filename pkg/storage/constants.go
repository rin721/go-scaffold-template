package storage

// 本文件属于存储抽象层，统一本地/内存文件系统、复制、监听、MIME、Excel 与图片辅助能力。

// FSType 定义文件系统类型
type FSType string

// Driver selects the high-level storage strategy.
type Driver string

// ObjectProvider selects the S3-compatible provider preset.
type ObjectProvider string

const (
	// FSTypeOS 使用操作系统原生文件系统
	FSTypeOS FSType = "os"

	// FSTypeMemory 使用内存文件系统(用于测试)
	FSTypeMemory FSType = "memory"

	// FSTypeReadOnly 使用只读文件系统
	FSTypeReadOnly FSType = "readonly"

	// FSTypeBasePathFS 使用带基础路径的文件系统
	FSTypeBasePathFS FSType = "basepath"
)

const (
	DriverDisabled   Driver = "disabled"
	DriverLocal      Driver = "local"
	DriverS3         Driver = "s3"
	DriverMinIO      Driver = "minio"
	DriverLocalS3    Driver = "local+s3"
	DriverLocalMinIO Driver = "local+minio"
)

const (
	ObjectProviderS3    ObjectProvider = "s3"
	ObjectProviderR2    ObjectProvider = "r2"
	ObjectProviderMinIO ObjectProvider = "minio"
)

// 默认配置值
const (
	// DefaultBasePath 默认基础路径
	DefaultBasePath = "."

	// DefaultFSType 默认文件系统类型
	DefaultFSType = FSTypeOS
)

// 文件监听事件类型
const (
	// WatchEventCreate 文件创建事件
	WatchEventCreate = "CREATE"

	// WatchEventWrite 文件写入事件
	WatchEventWrite = "WRITE"

	// WatchEventRemove 文件删除事件
	WatchEventRemove = "REMOVE"

	// WatchEventRename 文件重命名事件
	WatchEventRename = "RENAME"

	// WatchEventChmod 文件权限变更事件
	WatchEventChmod = "CHMOD"
)
