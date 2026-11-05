package admin

import (
	"context"

	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

// 安全配置仓储（单行表 sys_security_config，gva sys_security_config 迁入）。
type securityConfigRepo struct {
	query *dao.Query
	log   *log.Helper
}

var _ admin.SecurityConfigRepo = (*securityConfigRepo)(nil)

// NewSecurityConfigRepo 安全配置仓储
func NewSecurityConfigRepo(query *dao.Query, logger log.Logger) admin.SecurityConfigRepo {
	return &securityConfigRepo{query: query, log: log.NewHelper(log.With(logger, "module", "data/security_config"))}
}

// FindByID 单行表按固定主键取；不存在返回 gorm.ErrRecordNotFound（用例据此补默认行）。
func (r *securityConfigRepo) FindByID(ctx context.Context, id int64) (*model.SysSecurityConfig, error) {
	q := r.query.SysSecurityConfig
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (r *securityConfigRepo) Create(ctx context.Context, cfg *model.SysSecurityConfig) error {
	return r.query.SysSecurityConfig.WithContext(ctx).Create(cfg)
}

func (r *securityConfigRepo) Save(ctx context.Context, cfg *model.SysSecurityConfig) error {
	return r.query.SysSecurityConfig.WithContext(ctx).Save(cfg)
}
