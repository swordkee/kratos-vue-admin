package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

type sysLogsRepo struct {
	query *dao.Query
	log   *log.Helper
}

func NewSysLogsRepo(query *dao.Query, logger log.Logger) admin.SysLogsRepo {
	return &sysLogsRepo{query: query, log: log.NewHelper(logger)}
}

func (s *sysLogsRepo) Create(ctx context.Context, g *model.SysLogs) error {
	q := s.query.SysLogs
	return q.WithContext(ctx).Create(g)
}

func (s *sysLogsRepo) FindByID(ctx context.Context, id int64) (*model.SysLogs, error) {
	q := s.query.SysLogs
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (r *sysLogsRepo) Find(ctx context.Context, offset, limit int) ([]*model.SysLogs, error) {
	var records []*model.SysLogs
	err := r.query.WithContext(ctx).
		Offset(offset).
		Limit(limit).
		Order("id DESC").
		Find(&records).Error

	return records, err
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

func (s *sysLogsRepo) Delete(ctx context.Context, id int64) error {
	q := s.query.SysLogs
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

// DeleteByIds deletes operation records by ids
func (s *sysLogsRepo) DeleteByIds(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := s.query.SysLogs
	return q.WithContext(ctx).
		Where("id IN ?", ids).
		Delete(&model.SysLogs{}).Error
}

// DeleteByTimeRange deletes all operation records within the specified time range
func (s *sysLogsRepo) DeleteByTimeRange(ctx context.Context, startTime, endTime string) error {
	q := s.query.SysLogs
	return q.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Unscoped().
		Delete(&model.SysLogs{})
}

// FindByTimeRange finds operation records within the specified time range
func (r *sysLogsRepo) FindByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*model.SysLogs, int64, error) {
	var records []*model.SysLogs
	var count int64

	// Get total count within time range
	err := r.query.WithContext(ctx).
		Model(&model.SysLogs{}).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Count(&count).Error

	if err != nil {
		return nil, 0, err
	}

	// Get records within time range
	err = r.query.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", startTime, endTime).
		Offset(offset).
		Limit(limit).
		Order("id DESC").
		Find(&records).Error

	return records, count, err
}
