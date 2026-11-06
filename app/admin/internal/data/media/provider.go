package media

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/wire"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
)

// ProviderSet 媒体库数据层 provider（三组仓储 + 存储 + 分片暂存盘）。
var ProviderSet = wire.NewSet(
	NewCategoryRepo,
	NewFileRepo,
	NewChunkRepo,
	NewStorageRepo,
	NewChunkStore,
)

// NewChunkStore 分片暂存盘：落在本地存储目录下的 .chunks/（aliyun 模式同样先落本地再上传；
// 目录取 conf.oss.local.dir，缺省 uploads —— 不依赖进程工作目录以外的可写位置）。
func NewChunkStore(c *conf.Oss) *media.ChunkStore {
	base := "uploads"
	if c != nil && c.Local != nil && c.Local.Dir != "" {
		base = c.Local.Dir
	}
	return media.NewChunkStore(filepath.Join(base, ".chunks"))
}

// NewStorageRepo 按 conf.oss.use 选择 local / aliyun 实现（配置不全时装配期 fail-closed）。
func NewStorageRepo(c *conf.Oss, logger log.Logger) (media.StorageRepo, error) {
	if c == nil {
		return nil, errors.New("oss 配置缺失")
	}
	switch c.Use {
	case conf.OssUseMode_local:
		dir := ""
		if c.Local != nil {
			dir = c.Local.Dir
		}
		if dir == "" {
			return nil, errors.New("oss.local.dir 未配置")
		}
		return newLocalStorage(dir, logger), nil
	case conf.OssUseMode_aliyun:
		if c.Aliyun == nil {
			return nil, errors.New("oss.aliyun 未配置")
		}
		return newAliyunStorage(c.Aliyun, logger)
	default:
		return nil, fmt.Errorf("未知的 oss.use: %v", c.Use)
	}
}
