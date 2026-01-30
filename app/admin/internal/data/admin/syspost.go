package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"

	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

type sysPostRepo struct {
	query *dao.Query
	log   *log.Helper
}

func NewSysPostRepo(query *dao.Query, logger log.Logger) admin.SysPostRepo {
	return &sysPostRepo{
		query: query,
		log:   log.NewHelper(logger),
	}
}

func (p *sysPostRepo) Create(ctx context.Context, post *model.SysPosts) error {
	q := p.query.SysPosts
	return q.WithContext(ctx).Create(post)
}

func (p *sysPostRepo) Save(ctx context.Context, post *model.SysPosts) error {
	q := p.query.SysPosts
	return q.WithContext(ctx).Save(post)
}

func (p *sysPostRepo) Delete(ctx context.Context, ids []int64) error {
	q := p.query.SysPosts
	_, err := q.WithContext(ctx).Where(q.ID.In(ids...)).Delete()
	return err
}

func (p *sysPostRepo) FindByID(ctx context.Context, id int64) (*model.SysPosts, error) {
	q := p.query.SysPosts
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (p *sysPostRepo) ListPage(ctx context.Context, postName, postCode string, status int32, page, size int32) ([]*model.SysPosts, error) {
	q := p.query.SysPosts
	db := q.WithContext(ctx)
	if postName != "" {
		db = db.Where(q.PostName.Like(buildLikeValue(postName)))
	}
	if postCode != "" {
		db = db.Where(q.PostCode.Eq(postCode))
	}
	if status != 0 {
		db = db.Where(q.Status.Eq(status))
	}
	limit, offset := convertPageSize(page, size)
	return db.Limit(limit).Offset(offset).Find()
}

func (p *sysPostRepo) ListPageCount(ctx context.Context, postName, postCode string, status int32) (int32, error) {
	q := p.query.SysPosts
	db := q.WithContext(ctx)
	if postName != "" {
		db = db.Where(q.PostName.Like(buildLikeValue(postName)))
	}
	if postCode != "" {
		db = db.Where(q.PostCode.Eq(postCode))
	}
	if status != 0 {
		db = db.Where(q.Status.Eq(status))
	}
	count, err := db.Count()
	return int32(count), err
}

func (p *sysPostRepo) FindByIDList(ctx context.Context, ids ...int64) ([]*model.SysPosts, error) {
	q := p.query.SysPosts
	return q.WithContext(ctx).Where(q.ID.In(ids...)).Find()
}

func (p *sysPostRepo) FindAll(ctx context.Context) ([]*model.SysPosts, error) {
	q := p.query.SysPosts
	return q.WithContext(ctx).Find()
}
