package utils

import (
	"io"
	"os"
)

// CopyFile 拷贝文件
func CopyFile(srcFile, descFile string) error {
	// 1. 打开源文件（只读模式）
	src, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	// 延迟关闭源文件，避免资源泄漏
	defer src.Close()

	// 2. 创建/打开目标文件（写入模式，不存在则创建，存在则覆盖）
	// 权限 0644：所有者可读写，其他用户只读
	dst, err := os.Create(descFile)
	if err != nil {
		return err
	}
	// 延迟关闭目标文件
	defer dst.Close()

	// 3. 核心拷贝逻辑：使用 io.Copy 拷贝文件内容
	// io.Copy 是 Go 内置的核心拷贝函数，会自动处理缓冲区
	_, err = io.Copy(dst, src)
	if err != nil {
		return err
	}

	// 4. 确保数据刷入磁盘（可选，但推荐）
	err = dst.Sync()
	return err
}
