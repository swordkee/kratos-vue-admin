package media

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
)

// localStorage 本地存储：对象落在 conf.oss.local.dir，访问地址走 /static/ 前缀
// （server/http.go 注册的公开静态路由，对齐源端 gin static 语义）。
type localStorage struct {
	dir string
	l   *log.Helper
}

var _ media.StorageRepo = (*localStorage)(nil)

func newLocalStorage(dir string, logger log.Logger) media.StorageRepo {
	return &localStorage{
		dir: dir,
		l:   log.NewHelper(log.With(logger, "module", "app/admin/internal/data/media/local")),
	}
}

func (s *localStorage) Mode() string { return string(conf.OssUseMode_local) }

func (s *localStorage) URL(key string) string { return "/static/" + key }

// fullPath key → 绝对路径；两次防护：ValidateKey（拒绝 .. 与 Windows 盘符）+ Rel 校验（必须仍在根内）。
func (s *localStorage) fullPath(key string) (string, error) {
	if err := media.ValidateKey(key); err != nil {
		return "", err
	}
	p := filepath.Join(s.dir, filepath.FromSlash(path.Clean(key)))
	root, err := filepath.Abs(s.dir)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil || rel == ".." || len(rel) > 3 && rel[:3] == ".."+string(os.PathSeparator) {
		return "", media.ErrInvalidFileName
	}
	return abs, nil
}

func (s *localStorage) Save(_ context.Context, key string, r io.Reader, _ int64) error {
	p, err := s.fullPath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	out, err := os.Create(p)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, r)
	return err
}

func (s *localStorage) Open(_ context.Context, key string) (io.ReadCloser, error) {
	p, err := s.fullPath(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, media.ErrObjectNotFound
		}
		return nil, err
	}
	return f, nil
}

func (s *localStorage) Delete(_ context.Context, key string) error {
	p, err := s.fullPath(key)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil {
		if os.IsNotExist(err) {
			return media.ErrObjectNotFound
		}
		return err
	}
	return nil
}
