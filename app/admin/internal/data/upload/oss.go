package data

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/upload"
)

type OssRepo struct {
	log  *log.Helper
	path string
}

var _ upload.OssRepo = (*OssRepo)(nil)

func NewOssRepo(path string, logger log.Logger) upload.OssRepo {
	return &OssRepo{
		log:  log.NewHelper(logger),
		path: path,
	}
}

func (o *OssRepo) UploadFile(file multipart.File, filePath string) (string, error) {
	var fileBytes []byte
	var fileName string

	// 读取文件内容
	var err error
	fileBytes, err = io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	defer file.Close()

	// 生成文件名
	if filePath == "" {
		fileName = fmt.Sprintf("%d", time.Now().UnixNano())
	} else {
		fileName = filepath.Base(filePath)
	}

	// 确保目录存在
	fullPath := filepath.Join(o.path, fileName)
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, fileBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fileName, nil
}

// Upload 实现 admin.OssRepo 接口中的 Upload 方法（别名）
func (o *OssRepo) Upload(ctx context.Context, file []byte) (string, error) {
	fileName := fmt.Sprintf("%d", time.Now().UnixNano())
	fullPath := filepath.Join(o.path, fileName)

	// 确保目录存在
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(fullPath, file, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fileName, nil
}
