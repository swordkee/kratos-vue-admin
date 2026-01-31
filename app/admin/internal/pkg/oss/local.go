package oss

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
)

type localClient struct {
	log *log.Helper
	dir string
}

func newLocalClient(log *log.Helper, config *conf.OssLocalConfig) (*localClient, error) {
	return &localClient{log: log, dir: config.Dir}, nil
}

func (c *localClient) UploadFile(file interface{}, path string) (string, error) {
	reader, ok := file.(io.Reader)
	if !ok {
		return "", errors.New("file must be io.Reader")
	}
	path = filepath.Join(c.dir, path)
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0744); err != nil {
		return "", err
	}
	out, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer out.Close()

	_, err = io.Copy(out, reader)
	return "/", err
}

func (c *localClient) Upload(ctx context.Context, file []byte) (string, error) {
	return "/", errors.New("not implemented")
}
