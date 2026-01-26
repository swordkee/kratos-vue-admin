package biz

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"

	"github.com/go-kratos/kratos/v2/log"
)

// SysLogsRepo is a Greater repo.
type SysLogsRepo interface {
	Create(ctx context.Context, g *model.SysLogs) (*model.SysLogs, error)
	First(ctx context.Context, id int64) (*model.SysLogs, error)
	Find(ctx context.Context, offset, limit int) ([]*model.SysLogs, error)
	Count(ctx context.Context) (int64, error)
	FindByPage(ctx context.Context, offset, limit int) (result []*model.SysLogs, count int64, err error)
	Delete(ctx context.Context, id int64) error
	DeleteByIds(ctx context.Context, ids []int64) error
	DeleteByTimeRange(ctx context.Context, startTime, endTime string) error
	FindByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*model.SysLogs, int64, error)
}

// SysLogsUseCase is a SysOperationRecords use case.
type SysLogsUseCase struct {
	opRepo SysLogsRepo
	log    log.Logger
}

// NewSysLogsUseCase new a SysOperationRecords use case.
func NewSysLogsUseCase(opRepo SysLogsRepo, logger log.Logger) *SysLogsUseCase {
	return &SysLogsUseCase{
		opRepo: opRepo,
		log:    logger,
	}
}

// CreateOperationRecord creates a SysOperationRecords, and returns the new SysOperationRecords.
func (uc *SysLogsUseCase) CreateOperationRecord(ctx context.Context, g *model.SysLogs) (*model.SysLogs, error) {
	return uc.opRepo.Create(ctx, g)
}

// FindOperationRecordById finds a SysOperationRecords by id.
func (uc *SysLogsUseCase) FindOperationRecordById(ctx context.Context, id int64) (*model.SysLogs, error) {
	return uc.opRepo.First(ctx, id)
}

// ListPage lists SysOperationRecords by page.
func (uc *SysLogsUseCase) ListPage(ctx context.Context, pageNum, pageSize int32) ([]*model.SysLogs, int64, error) {
	return uc.opRepo.FindByPage(ctx, int((pageNum-1)*pageSize), int(pageSize))
}

// DeleteOperationRecord deletes a SysOperationRecords by id.
func (uc *SysLogsUseCase) DeleteOperationRecord(ctx context.Context, id int64) error {
	return uc.opRepo.Delete(ctx, id)
}

// DeleteByIds deletes operation records by ids.
func (uc *SysLogsUseCase) DeleteByIds(ctx context.Context, ids []int64) error {
	return uc.opRepo.DeleteByIds(ctx, ids)
}

// DeleteByTimeRange deletes operation records within the specified time range
func (uc *SysLogsUseCase) DeleteByTimeRange(ctx context.Context, startTime, endTime string) error {
	return uc.opRepo.DeleteByTimeRange(ctx, startTime, endTime)
}

// FindByTimeRange finds operation records within the specified time range
func (uc *SysLogsUseCase) FindByTimeRange(ctx context.Context, startTime, endTime string, offset, limit int) ([]*model.SysLogs, int64, error) {
	return uc.opRepo.FindByTimeRange(ctx, startTime, endTime, offset, limit)
}
