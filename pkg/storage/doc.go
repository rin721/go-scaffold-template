// Package storage 提供项目自有存储契约和文件辅助能力。
//
// # 设计目标
//
//   - 提供统一的文件操作和对象存储接口，屏蔽底层实现细节。
//   - 支持多种本地文件系统类型、文件监听、复制、MIME、Excel 和图片辅助。
//   - 通过 StorageClient 接入 local、S3、R2、MinIO 等对象存储。
//   - 公开 API 使用项目自有类型，不把第三方库类型泄漏给业务常用接口。
//
// # 使用示例
//
// 基础文件操作:
//
//	cfg := &storage.Config{Local: storage.LocalConfig{FSType: storage.FSTypeMemory}}
//	fs, err := storage.New(cfg)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer fs.Close()
//
//	// 读写文件
//	err = fs.WriteFile("test.txt", []byte("hello"), 0644)
//	data, err := fs.ReadFile("test.txt")
//
// 文件复制:
//
//	// 复制单个文件
//	err = fs.Copy("source.txt", "dest.txt")
//
//	// 复制目录
//	err = fs.CopyDir("./source_dir", "./dest_dir",
//	    storage.WithPreserveTimes(true))
//
// MIME检测:
//
//	mimeType, err := fs.DetectMIME("image.jpg")
//	fmt.Println(mimeType) // "image/jpeg"
//
// 文件监听:
//
//	err = fs.Watch("./watch_dir", func(event storage.WatchEvent) {
//	    fmt.Printf("File %s: %s\n", event.Op, event.Path)
//	})
//	defer fs.StopWatch("./watch_dir")
//
// Excel处理:
//
//	// 读取Excel
//	rows, err := fs.ReadExcelSheet("data.xlsx", "Sheet1")
//
//	// 创建Excel
//	file := fs.CreateWorkbook()
//	file.SetCellValue("Sheet1", "A1", "Hello")
//	err = fs.SaveWorkbook(file, "output.xlsx")
//
// 图片处理:
//
//	// 调整图片大小
//	err = fs.ResizeImage("input.jpg", "output.jpg", 800, 600, storage.ImageFormatJPEG)
//
//	// 裁剪图片
//	rect := image.Rect(0, 0, 200, 200)
//	err = fs.CropImage("input.jpg", "cropped.jpg", rect, storage.ImageFormatPNG)
//
// # 接口设计
//
//   - Storage: 主接口,提供所有文件操作方法
//   - WatchHandler: 文件监听事件处理函数
//   - CopyOption: 文件复制选项接口
//
// # 线程安全
//
// 所有方法都是并发安全的,使用 sync.RWMutex 保护内部状态。
package storage

// 本文件承载包级 Godoc 入口，集中说明该包在脚手架架构中的定位、使用边界和非目标能力。
