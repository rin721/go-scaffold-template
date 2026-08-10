package storage

// 本测试文件固定存储抽象的文件、复制、监听和媒体辅助能力，防止注释补全和后续重构改变外部可观察行为。

import (
	"errors"
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMemoryFileOperations 固定存储抽象的文件、复制、监听和媒体辅助能力，确保后续注释补全或结构调整不改变该场景。
func TestMemoryFileOperations(t *testing.T) {
	fs := newMemoryStorage(t)
	defer fs.Close()

	if err := fs.MkdirAll("docs", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := fs.WriteFile("docs/readme.txt", []byte("hello"), 0644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	data, err := fs.ReadFile("docs/readme.txt")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("ReadFile() = %q, want hello", data)
	}
	if exists, err := fs.Exists("docs/readme.txt"); err != nil || !exists {
		t.Fatalf("Exists(file) = %v, %v; want true, nil", exists, err)
	}
	if isFile, err := fs.IsFile("docs/readme.txt"); err != nil || !isFile {
		t.Fatalf("IsFile() = %v, %v; want true, nil", isFile, err)
	}
	if isDir, err := fs.IsDir("docs"); err != nil || !isDir {
		t.Fatalf("IsDir() = %v, %v; want true, nil", isDir, err)
	}
	if size, err := fs.FileSize("docs/readme.txt"); err != nil || size != 5 {
		t.Fatalf("FileSize() = %d, %v; want 5, nil", size, err)
	}
	if entries, err := fs.ListDir("docs"); err != nil || len(entries) != 1 {
		t.Fatalf("ListDir() len = %d, err = %v; want 1, nil", len(entries), err)
	}
	if err := fs.Remove("docs/readme.txt"); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if exists, err := fs.Exists("docs/readme.txt"); err != nil || exists {
		t.Fatalf("Exists(removed) = %v, %v; want false, nil", exists, err)
	}
	if err := fs.RemoveAll("docs"); err != nil {
		t.Fatalf("RemoveAll() error = %v", err)
	}
}

// TestCopyAndCopyDir 固定存储抽象的文件、复制、监听和媒体辅助能力，确保后续注释补全或结构调整不改变该场景。
func TestCopyAndCopyDir(t *testing.T) {
	fs := newMemoryStorage(t)
	defer fs.Close()

	if err := fs.MkdirAll("src/sub", 0755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	mustWriteFile(t, fs, "src/a.txt", "alpha")
	mustWriteFile(t, fs, "src/sub/b.txt", "beta")
	mustWriteFile(t, fs, "src/skip.tmp", "skip")

	if err := fs.Copy("src/a.txt", "single.txt", WithPreserveTimes(true)); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	assertFileContent(t, fs, "single.txt", "alpha")

	if err := fs.CopyDir("src", "dst", WithSkip(func(path string) bool {
		return strings.HasSuffix(path, "skip.tmp")
	})); err != nil {
		t.Fatalf("CopyDir() error = %v", err)
	}
	assertFileContent(t, fs, "dst/a.txt", "alpha")
	assertFileContent(t, fs, "dst/sub/b.txt", "beta")
	if exists, err := fs.Exists("dst/skip.tmp"); err != nil || exists {
		t.Fatalf("Exists(skipped) = %v, %v; want false, nil", exists, err)
	}

	if err := fs.Copy("missing.txt", "out.txt"); !errors.Is(err, ErrPathNotFound) {
		t.Fatalf("Copy(missing) error = %v, want ErrPathNotFound", err)
	}
	if err := fs.Copy("src", "out.txt"); !errors.Is(err, ErrNotFile) {
		t.Fatalf("Copy(directory) error = %v, want ErrNotFile", err)
	}
	if err := fs.CopyDir("src/a.txt", "out"); !errors.Is(err, ErrNotDirectory) {
		t.Fatalf("CopyDir(file) error = %v, want ErrNotDirectory", err)
	}
}

// TestMIMEExcelAndImageHelpers 固定存储抽象的文件、复制、监听和媒体辅助能力，确保后续注释补全或结构调整不改变该场景。
func TestMIMEExcelAndImageHelpers(t *testing.T) {
	fs := newMemoryStorage(t)
	defer fs.Close()

	mimeType, err := fs.DetectMIMEFromBytes([]byte("hello"))
	if err != nil {
		t.Fatalf("DetectMIMEFromBytes() error = %v", err)
	}
	if !strings.HasPrefix(mimeType, "text/plain") {
		t.Fatalf("DetectMIMEFromBytes() = %q, want text/plain", mimeType)
	}

	file := fs.CreateWorkbook()
	if err := file.SetCellValue("Sheet1", "A1", "name"); err != nil {
		t.Fatalf("SetCellValue() error = %v", err)
	}
	if err := fs.SaveWorkbook(file, "book.xlsx"); err != nil {
		t.Fatalf("SaveWorkbook() error = %v", err)
	}
	rows, err := fs.ReadExcelSheet("book.xlsx", "Sheet1")
	if err != nil {
		t.Fatalf("ReadExcelSheet() error = %v", err)
	}
	if rows[0][0] != "name" {
		t.Fatalf("ReadExcelSheet()[0][0] = %q, want name", rows[0][0])
	}

	img := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 20), G: uint8(y * 20), B: 200, A: 255})
		}
	}
	if err := fs.SaveImage(img, "image.png", ImageFormatPNG); err != nil {
		t.Fatalf("SaveImage() error = %v", err)
	}
	fileMIME, err := fs.DetectMIME("image.png")
	if err != nil {
		t.Fatalf("DetectMIME() error = %v", err)
	}
	if fileMIME != "image/png" {
		t.Fatalf("DetectMIME() = %q, want image/png", fileMIME)
	}
	if err := fs.ResizeImage("image.png", "small.png", 4, 0, ImageFormatPNG); err != nil {
		t.Fatalf("ResizeImage() error = %v", err)
	}
	resized, err := fs.OpenImage("small.png")
	if err != nil {
		t.Fatalf("OpenImage(resized) error = %v", err)
	}
	if resized.Bounds().Dx() != 4 {
		t.Fatalf("resized width = %d, want 4", resized.Bounds().Dx())
	}
	if err := fs.CropImage("image.png", "crop.png", image.Rect(0, 0, 3, 2), ImageFormatPNG); err != nil {
		t.Fatalf("CropImage() error = %v", err)
	}
	cropped, err := fs.OpenImage("crop.png")
	if err != nil {
		t.Fatalf("OpenImage(cropped) error = %v", err)
	}
	if cropped.Bounds().Dx() != 3 || cropped.Bounds().Dy() != 2 {
		t.Fatalf("cropped size = %dx%d, want 3x2", cropped.Bounds().Dx(), cropped.Bounds().Dy())
	}
}

// TestConfigValidationAndDisabledWatch 固定存储配置校验和禁用监听的语义。
func TestConfigValidationAndDisabledWatch(t *testing.T) {
	var cfg Config
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(empty) error = %v, want nil default local config", err)
	}

	cfg = Config{Local: LocalConfig{FSType: FSType("invalid")}}
	if err := cfg.Validate(); !errors.Is(err, ErrInvalidFSType) {
		t.Fatalf("Validate(invalid fs type) error = %v, want ErrInvalidFSType", err)
	}

	fs := newMemoryStorage(t)
	defer fs.Close()
	if err := fs.Watch("missing", func(WatchEvent) {}); err == nil {
		t.Fatal("Watch(disabled) error = nil, want error")
	}
}

func TestWatchErrorEventCarriesCause(t *testing.T) {
	fs, err := New(&Config{Local: LocalConfig{FSType: FSTypeOS, EnableWatch: true}})
	if err != nil {
		t.Fatalf("New(watch enabled) error = %v", err)
	}
	defer fs.Close()

	impl, ok := fs.(*impl)
	if !ok {
		t.Fatalf("New() returned %T, want *impl", fs)
	}

	dir := t.TempDir()
	events := make(chan WatchEvent, 1)
	if err := fs.Watch(dir, func(event WatchEvent) {
		events <- event
	}); err != nil {
		t.Fatalf("Watch() error = %v", err)
	}

	cause := errors.New("watch backend unavailable")
	sent := make(chan struct{})
	go func() {
		impl.watcher.Errors <- cause
		close(sent)
	}()

	select {
	case event := <-events:
		if event.Op != "ERROR" {
			t.Fatalf("event Op = %q, want ERROR", event.Op)
		}
		if !errors.Is(event.Error, cause) {
			t.Fatalf("event Error = %v, want %v", event.Error, cause)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for watch error event")
	}
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("timed out sending watch error")
	}

	if err := fs.StopWatch(dir); err != nil {
		t.Fatalf("StopWatch() error = %v", err)
	}
}

// newMemoryStorage 构造当前测试场景所需的最小依赖集合，避免测试直接耦合生产装配流程。
func newMemoryStorage(t *testing.T) Storage {
	t.Helper()
	fs, err := New(&Config{Local: LocalConfig{FSType: FSTypeMemory, EnableWatch: false}})
	if err != nil {
		t.Fatalf("New(memory) error = %v", err)
	}
	return fs
}

// mustWriteFile 是当前测试文件的辅助函数，用于复用夹具、断言或输入构造逻辑。
func mustWriteFile(t *testing.T, fs Storage, path, content string) {
	t.Helper()
	if err := fs.WriteFile(path, []byte(content), os.FileMode(0644)); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

// assertFileContent 校验测试响应或状态中的关键字段，使测试断言聚焦在对外契约而非重复解析细节。
func assertFileContent(t *testing.T, fs Storage, path, want string) {
	t.Helper()
	data, err := fs.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("ReadFile(%s) = %q, want %q", path, data, want)
	}
}
