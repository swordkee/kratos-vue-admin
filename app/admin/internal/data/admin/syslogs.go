package admin

import (
	"context"
	"time"

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

func (r *sysLogsRepo) Count(ctx context.Context) (int64, error) {
	q := r.query.SysLogs
	count, err := q.WithContext(ctx).Count()
	return count, err
}

func (r *sysLogsRepo) Find(ctx context.Context, offset, limit int) ([]*model.SysLogs, error) {
	q := r.query.SysLogs
	return q.WithContext(ctx).Limit(limit).Offset(offset).Find()
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
func (r *sysLogsRepo) DeleteByIds(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := r.query.SysLogs
	_, err := q.WithContext(ctx).Where(q.ID.In(ids...)).Delete()
	return err
}

// DeleteByTimeRange deletes all operation records within the specified time range
func (s *sysLogsRepo) DeleteByTimeRange(ctx context.Context, startTime, endTime string) error {
	start, err1 := time.Parse("2006-01-02 15:04:05", startTime)
	if err1 != nil {
		return err1
	}
	end, err2 := time.Parse("2006-01-02 15:04:05", endTime)
	if err2 != nil {
		return err2
	}

	q := s.query.SysLogs
	_, err := q.WithContext(ctx).
		Where(q.CreatedAt.Gte(start), q.CreatedAt.Lte(end)).
		Delete()
	return err
}

// FindByTimeRange finds operation records within the specified time range
func (r *sysLogsRepo) FindByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*model.SysLogs, int64, error) {
	start, err1 := time.Parse("2006-01-02 15:04:05", startTime)
	if err1 != nil {
		return nil, 0, err1
	}
	end, err2 := time.Parse("2006-01-02 15:04:05", endTime)
	if err2 != nil {
		return nil, 0, err2
	}

	var records []*model.SysLogs
	var count int64

	q := r.query.SysLogs
	condition := q.WithContext(ctx).Where(q.CreatedAt.Gte(start), q.CreatedAt.Lte(end))

	// Get total count within time range
	count, err := condition.Count()
	if err != nil {
		return nil, 0, err
	}

	// Get records within time range
	records, err = condition.Offset(offset).Limit(limit).Order(q.ID.Desc()).Find()
	return records, count, err
}
