package data

import (
	"context"
	"errors"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"gorm.io/gorm"
)

type sysLogsRepo struct {
	data *Data
	log  *log.Helper
}

func NewSysLogsRepo(data *Data, logger log.Logger) biz.SysLogsRepo {
	return &sysLogsRepo{data: data, log: log.NewHelper(logger)}
}

func (r *sysLogsRepo) Create(ctx context.Context, g *model.SysLogs) (*model.SysLogs, error) {
	err := r.data.db.WithContext(ctx).Create(g).Error
	return g, err
}

func (r *sysLogsRepo) First(ctx context.Context, id int64) (*model.SysLogs, error) {
	var record model.SysLogs
	err := r.data.db.WithContext(ctx).
		Where("id = ?", id).
		First(&record).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &record, nil
}

func (r *sysLogsRepo) Find(ctx context.Context, offset, limit int) ([]*model.SysLogs, error) {
	var records []*model.SysLogs
	err := r.data.db.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("id DESC").
		Find(&records).Error

	return records, err
}

func (r *sysLogsRepo) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.data.db.WithContext(ctx).
		Model(&model.SysLogs{}).
		Count(&count).Error

	return count, err
}

func (r *sysLogsRepo) FindByPage(ctx context.Context, offset, limit int) (result []*model.SysLogs, count int64, err error) {
	// Get total count
	count, err = r.Count(ctx)
	if err != nil {
		return nil, 0, err
	}

	// Get records
	result, err = r.Find(ctx, offset, limit)
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}

func (r *sysLogsRepo) Delete(ctx context.Context, id int64) error {
	return r.data.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.SysLogs{}).Error
}

// DeleteByIds deletes operation records by ids
func (r *sysLogsRepo) DeleteByIds(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	return r.data.db.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&model.SysLogs{}).Error
}

// DeleteByTimeRange deletes all operation records within the specified time range
func (r *sysLogsRepo) DeleteByTimeRange(ctx context.Context, startTime, endTime string) error {
	return r.data.db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Unscoped().
		Delete(&model.SysLogs{}).Error
}

// FindByTimeRange finds operation records within the specified time range
func (r *sysLogsRepo) FindByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*model.SysLogs, int64, error) {
	var records []*model.SysLogs
	var count int64

	// Get total count within time range
	err := r.data.db.WithContext(ctx).
		Model(&model.SysLogs{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Count(&count).Error

	if err != nil {
		return nil, 0, err
	}

	// Get records within time range
	err = r.data.db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Offset(offset).
		Limit(limit).
		Order("id DESC").
		Find(&records).Error

	return records, count, err
}
