package data

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

type sysuserRepo struct {
	data *Data
	log  *log.Helper
}

// NewSysUserRepo .
func NewSysUserRepo(data *Data, logger log.Logger) biz.SysUserRepo {
	return &sysuserRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *sysuserRepo) Create(ctx context.Context, g *model.SysUsers) (*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	err := q.WithContext(ctx).Clauses().Create(g)
	return g, err
}

func (r *sysuserRepo) Save(ctx context.Context, g *model.SysUsers) (*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	err := q.WithContext(ctx).Clauses().Save(g)
	return g, err
}

func (r *sysuserRepo) Delete(ctx context.Context, id int64) error {
	q := r.data.Query(ctx).SysUsers
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

func (r *sysuserRepo) FindByID(ctx context.Context, id int64) (*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (r *sysuserRepo) FindAll(ctx context.Context) ([]*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	return q.WithContext(ctx).Find()
}

func (r *sysuserRepo) FindByUsername(ctx context.Context, username string) (*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	return q.WithContext(ctx).Where(q.Username.Eq(username)).First()
}

func (r *sysuserRepo) ListPage(ctx context.Context, page, size int32, condition biz.UserListCondition) ([]*model.SysUsers, error) {
	m := r.data.Query(ctx).SysUsers
	q := m.WithContext(ctx)
	if condition.Status != 0 {
		q = q.Where(m.Status.Eq(condition.Status))
	}
	if condition.UserName != "" {
		q = q.Where(m.Username.Like("%" + condition.UserName + "%"))
	}
	if condition.Phone != "" {
		q = q.Where(m.Phone.Like("%" + condition.Phone + "%"))
	}
	limit, offset := convertPageSize(page, size)
	return q.Limit(limit).Offset(offset).Find()
}

func (r *sysuserRepo) Count(ctx context.Context, condition biz.UserListCondition) (int32, error) {
	m := r.data.Query(ctx).SysUsers
	q := m.WithContext(ctx)
	if condition.Status != 0 {
		q = q.Where(m.Status.Eq(condition.Status))
	}
	if condition.UserName != "" {
		q = q.Where(m.Username.Like("%" + condition.UserName + "%"))
	}
	if condition.Phone != "" {
		q = q.Where(m.Phone.Like("%" + condition.Phone + "%"))
	}
	count, err := q.Count()
	return int32(count), err
}

func (r *sysuserRepo) FindByPostId(ctx context.Context, postId int64) ([]*model.SysUsers, error) {
	q := r.data.Query(ctx).SysUsers
	return q.WithContext(ctx).Where(q.PostID.Eq(postId)).Find()
}

func (r *sysuserRepo) CountByRoleId(ctx context.Context, roleId int64) (int64, error) {
	q := r.data.Query(ctx).SysUsers
	return q.WithContext(ctx).Where(q.RoleID.Eq(roleId)).Count()
}

func (r *sysuserRepo) UpdateByID(ctx context.Context, id int64, user *model.SysUsers) error {
	if id == 0 {
		return errors.New("user can not update without id")
	}
	q := r.data.Query(ctx).SysUsers
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Updates(user)
	return err
}
