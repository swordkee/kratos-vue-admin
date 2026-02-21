package admin

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"

	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

type sysApiRepo struct {
	query *dao.Query
	log   *log.Helper
}

func NewSysApiRepo(query *dao.Query, logger log.Logger) admin.SysApiRepo {
	return &sysApiRepo{
		query: query,
		log:   log.NewHelper(logger),
	}
}

func (a *sysApiRepo) Create(ctx context.Context, api *model.SysApis) error {
	q := a.query.SysApis
	return q.WithContext(ctx).Create(api)
}

func (a *sysApiRepo) Save(ctx context.Context, api *model.SysApis) error {
	q := a.query.SysApis
	return q.WithContext(ctx).Save(api)
}

func (a *sysApiRepo) Delete(ctx context.Context, id int64) error {
	q := a.query.SysApis
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

func (a *sysApiRepo) ListPage(ctx context.Context, page, size int32) ([]*model.SysApis, error) {
	q := a.query.SysApis
	db := q.WithContext(ctx)

	limit, offset := convertPageSize(page, size)
	return db.Limit(limit).Offset(offset).Find()
}

func (a *sysApiRepo) ListPageCount(ctx context.Context) (int32, error) {
	q := a.query.SysApis
	db := q.WithContext(ctx)
	count, err := db.Count()
	return int32(count), err
}

func (a *sysApiRepo) FindAll(ctx context.Context) ([]*model.SysApis, error) {
	q := a.query.SysApis
	return q.WithContext(ctx).Find()
}

func (a *sysApiRepo) FindByID(ctx context.Context, id int64) (*model.SysApis, error) {
	q := a.query.SysApis
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}
func convertPageSize(page, size int32) (limit, offset int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	limit = int(size)
	offset = int((page - 1) * size)
	return
}
func buildLikeValue(key string) string {
	return fmt.Sprintf("%%%s%%", key)
}
