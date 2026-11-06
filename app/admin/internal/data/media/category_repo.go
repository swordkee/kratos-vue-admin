package media

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// CategoryRepo 实现 biz/media.CategoryRepo（exa_attachment_category）。
type CategoryRepo struct {
	qs QuerySource
}

var _ media.CategoryRepo = (*CategoryRepo)(nil)

// NewCategoryRepo 构造分类仓储。
func NewCategoryRepo(qs QuerySource) *CategoryRepo {
	return &CategoryRepo{qs: qs}
}

func (r *CategoryRepo) FindAll(ctx context.Context) ([]*model.ExaAttachmentCategory, error) {
	q := r.qs.Query(ctx).ExaAttachmentCategory
	return q.WithContext(ctx).Find()
}

// FindByNameAndPid 同父同名查重（excludeID>0 时排除自身，见 biz/media 差异 1）。
func (r *CategoryRepo) FindByNameAndPid(ctx context.Context, name string, pid, excludeID int64) (*model.ExaAttachmentCategory, error) {
	q := r.qs.Query(ctx).ExaAttachmentCategory
	db := q.WithContext(ctx).UnderlyingDB().Model(&model.ExaAttachmentCategory{}).
		Where("name = ? AND pid = ?", name, pid)
	if excludeID > 0 {
		db = db.Where("id <> ?", excludeID)
	}
	var row model.ExaAttachmentCategory
	if err := db.Order("id").First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (r *CategoryRepo) Create(ctx context.Context, c *model.ExaAttachmentCategory) error {
	return r.qs.Query(ctx).ExaAttachmentCategory.WithContext(ctx).Create(c)
}

// UpdateNamePid 对齐源 Updates(&ExaAttachmentCategory{Name, Pid})（只写两列）。
func (r *CategoryRepo) UpdateNamePid(ctx context.Context, id int64, name string, pid int64) error {
	q := r.qs.Query(ctx).ExaAttachmentCategory
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).UpdateColumnSimple(q.Name.Value(name), q.Pid.Value(int32(pid)))
	return err
}

func (r *CategoryRepo) CountChildren(ctx context.Context, pid int64) (int64, error) {
	q := r.qs.Query(ctx).ExaAttachmentCategory
	return q.WithContext(ctx).Where(q.Pid.Eq(int32(pid))).Count()
}

// DeleteHard Unscoped 物理删（源 Unscoped().Delete）。
func (r *CategoryRepo) DeleteHard(ctx context.Context, id int64) error {
	q := r.qs.Query(ctx).ExaAttachmentCategory
	_, err := q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).Delete()
	return err
}
