package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"

	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

type SysUserRepo struct {
	query *dao.Query
	db    *gorm.DB
	log   *log.Helper
}

// NewSysUserRepo .
func NewSysUserRepo(query *dao.Query, db *gorm.DB, logger log.Logger) admin.SysUserRepo {
	return &SysUserRepo{
		query: query,
		db:    db,
		log:   log.NewHelper(logger),
	}
}

func (r *SysUserRepo) Create(ctx context.Context, g *model.SysUsers) (*model.SysUsers, error) {
	q := r.query.SysUsers
	err := q.WithContext(ctx).Clauses().Create(g)
	return g, err
}

func (r *SysUserRepo) Save(ctx context.Context, g *model.SysUsers) (*model.SysUsers, error) {
	q := r.query.SysUsers
	err := q.WithContext(ctx).Clauses().Save(g)
	return g, err
}

func (r *SysUserRepo) Delete(ctx context.Context, id int64) error {
	q := r.query.SysUsers
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

func (r *SysUserRepo) FindByID(ctx context.Context, id int64) (*model.SysUsers, error) {
	q := r.query.SysUsers
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (r *SysUserRepo) FindAll(ctx context.Context) ([]*model.SysUsers, error) {
	q := r.query.SysUsers
	return q.WithContext(ctx).Find()
}

func (r *SysUserRepo) FindByUsername(ctx context.Context, username string) (*model.SysUsers, error) {
	q := r.query.SysUsers
	return q.WithContext(ctx).Where(q.Username.Eq(username)).First()
}

func (r *SysUserRepo) ListPage(ctx context.Context, page, size int32, condition admin.UserListCondition) ([]*model.SysUsers, error) {
	m := r.query.SysUsers
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

func (r *SysUserRepo) Count(ctx context.Context, condition admin.UserListCondition) (int32, error) {
	m := r.query.SysUsers
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

func (r *SysUserRepo) FindByPostId(ctx context.Context, postId int64) ([]*model.SysUsers, error) {
	q := r.query.SysUsers
	return q.WithContext(ctx).Where(q.PostID.Eq(postId)).Find()
}

func (r *SysUserRepo) CountByRoleId(ctx context.Context, roleId int64) (int64, error) {
	q := r.query.SysUsers
	return q.WithContext(ctx).Where(q.RoleID.Eq(roleId)).Count()
}

func (r *SysUserRepo) UpdateByID(ctx context.Context, id int64, user *model.SysUsers) error {
	if id == 0 {
		return fmt.Errorf("user can not update without id: %w", fmt.Errorf("invalid id"))
	}
	q := r.query.SysUsers
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Updates(user)
	return err
}

// ==================== JWT 黑名单相关方法 ====================

// AddJwtToBlacklist 将 JWT 加入黑名单
func (r *SysUserRepo) AddJwtToBlacklist(ctx context.Context, jwt string) error {
	blacklist := &model.JwtBlacklists{
		Jwt:       jwt,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	return r.query.JwtBlacklists.WithContext(ctx).Create(blacklist)
}

// IsJwtInBlacklist 检查 JWT 是否在黑名单中
func (r *SysUserRepo) IsJwtInBlacklist(ctx context.Context, jwt string) (bool, error) {
	q := r.query.JwtBlacklists
	count, err := q.WithContext(ctx).Where(q.Jwt.Eq(jwt)).Count()
	return count > 0, err
}

// CleanExpiredBlacklists 清理过期的黑名单记录
func (r *SysUserRepo) CleanExpiredBlacklists(ctx context.Context) error {
	q := r.query.JwtBlacklists
	// 清理 7 天前的记录
	expiredTime := time.Now().AddDate(0, 0, -7)
	_, err := q.WithContext(ctx).Where(q.CreatedAt.Lt(expiredTime)).Delete()
	return err
}

// ==================== IP 黑名单相关方法 ====================

// AddIpToBlacklist 将 IP 添加到黑名单
func (r *SysUserRepo) AddIpToBlacklist(ctx context.Context, ip string, reason string) error {
	return r.db.WithContext(ctx).Table("ip_blacklist").Create(map[string]interface{}{
		"ip":         ip,
		"reason":     reason,
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}).Error
}

// IsIpInBlacklist 检查 IP 是否在黑名单中
func (r *SysUserRepo) IsIpInBlacklist(ctx context.Context, ip string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("ip_blacklist").Where("ip = ? AND deleted_at IS NULL", ip).Count(&count).Error
	return count > 0, err
}

// RemoveIpFromBlacklist 将 IP 从黑名单中移除（软删除）
func (r *SysUserRepo) RemoveIpFromBlacklist(ctx context.Context, ip string) error {
	return r.db.WithContext(ctx).Table("ip_blacklist").Where("ip = ?", ip).Update("deleted_at", time.Now()).Error
}
