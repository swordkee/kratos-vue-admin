package media

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/media"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// FileRepo 实现 biz/media.FileRepo（exa_file_upload_and_downloads）。
type FileRepo struct {
	qs QuerySource
}

var _ media.FileRepo = (*FileRepo)(nil)

// NewFileRepo 构造文件记录仓储。
func NewFileRepo(qs QuerySource) *FileRepo {
	return &FileRepo{qs: qs}
}

func (r *FileRepo) Create(ctx context.Context, f *model.ExaFileUploadAndDownloads) error {
	return r.qs.Query(ctx).ExaFileUploadAndDownloads.WithContext(ctx).Create(f)
}

func (r *FileRepo) CreateBatch(ctx context.Context, files []*model.ExaFileUploadAndDownloads) error {
	if len(files) == 0 {
		return nil
	}
	return r.qs.Query(ctx).ExaFileUploadAndDownloads.WithContext(ctx).Create(files...)
}

func (r *FileRepo) FindByID(ctx context.Context, id int64) (*model.ExaFileUploadAndDownloads, error) {
	q := r.qs.Query(ctx).ExaFileUploadAndDownloads
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (r *FileRepo) CountByKey(ctx context.Context, key string) (int64, error) {
	q := r.qs.Query(ctx).ExaFileUploadAndDownloads
	return q.WithContext(ctx).Where(q.Key.Eq(key)).Count()
}

func (r *FileRepo) UpdateName(ctx context.Context, id int64, name string) error {
	q := r.qs.Query(ctx).ExaFileUploadAndDownloads
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).UpdateColumnSimple(q.Name.Value(name))
	return err
}

// DeleteHard Unscoped 物理删（源 Unscoped().Delete）。
func (r *FileRepo) DeleteHard(ctx context.Context, id int64) error {
	q := r.qs.Query(ctx).ExaFileUploadAndDownloads
	_, err := q.WithContext(ctx).Unscoped().Where(q.ID.Eq(id)).Delete()
	return err
}

// ListPage 分页（源：keyword 模糊 name，ORDER BY updated_at DESC）。
func (r *FileRepo) ListPage(ctx context.Context, cond media.FileListCondition) ([]*model.ExaFileUploadAndDownloads, int64, error) {
	q := r.qs.Query(ctx).ExaFileUploadAndDownloads
	db := q.WithContext(ctx).UnderlyingDB().Model(&model.ExaFileUploadAndDownloads{})
	if cond.Keyword != "" {
		db = db.Where("name LIKE ?", "%"+cond.Keyword+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*model.ExaFileUploadAndDownloads{}, 0, nil
	}
	limit, offset := pageLimitOffset(cond.Page, cond.PageSize)
	var rows []*model.ExaFileUploadAndDownloads
	if err := db.Order("updated_at DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}
