package admin

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
)

type SysLogsService struct {
	pb.UnimplementedLogsServiceServer
	opRecordsCase *biz.SysLogsUseCase
	log           *log.Helper
}

func NewSysLogsService(opRecordsCase *biz.SysLogsUseCase, logger log.Logger) *SysLogsService {
	return &SysLogsService{
		opRecordsCase: opRecordsCase,
		log:           log.NewHelper(log.With(logger, "module", "service/operation_records")),
	}
}

// FindLogs 获取单条操作记录
func (s *SysLogsService) FindLogs(ctx context.Context, req *pb.FindLogsRequest) (*pb.FindLogsReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	record, err := s.opRecordsCase.FindOperationRecordById(ctx, req.Id)
	if err != nil {
		s.log.Error(err)
		return nil, errors.InternalServer("OPERATION_RECORD_GET_FAILED", "failed to get operation record")
	}

	return &pb.FindLogsReply{
		Data: &pb.SysLogs{
			Id:        record.ID,
			CreatedAt: record.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: record.UpdatedAt.Format("2006-01-02 15:04:05"),
			UserId:    record.UserID,
			Ip:        record.IP,
			Method:    record.Method,
			Status:    int32(record.Status),
		},
	}, nil
}

// DeleteOperationRecordsByIds 批量删除操作记录
func (s *SysLogsService) DeleteLogsByIds(ctx context.Context, req *pb.DeleteLogsByIdsRequest) (*pb.DeleteLogsByIdsReply, error) {
	// Parse comma-separated IDs
	var ids []int64
	for _, idStr := range strings.Split(req.Ids, ",") {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			s.log.Error("invalid id: %s", idStr)
			continue
		}
		ids = append(ids, id)
	}

	if len(ids) == 0 {
		return &pb.DeleteLogsByIdsReply{}, nil
	}

	err := s.opRecordsCase.DeleteByIds(ctx, ids)
	if err != nil {
		s.log.Error(err)
		return nil, errors.InternalServer("OPERATION_RECORD_DELETE_FAILED", "failed to delete operation records")
	}

	return &pb.DeleteLogsByIdsReply{}, nil
}

// ListSysOperationRecords 获取系统操作记录列表（详细版）
func (s *SysLogsService) ListLogs(ctx context.Context, req *pb.ListLogsRequest) (*pb.ListLogsReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	result, count, err := s.opRecordsCase.ListPage(ctx, req.PageNum, req.PageSize)
	if err != nil {
		s.log.Error(err)
		return nil, errors.InternalServer("OPERATION_RECORD_LIST_FAILED", "failed to list operation records")
	}

	replyList := make([]*pb.SysLogsDetail, len(result))
	for i, d := range result {
		replyList[i] = &pb.SysLogsDetail{
			Id:           d.ID,
			CreatedAt:    d.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt:    d.UpdatedAt.Format("2006-01-02 15:04:05"),
			Ip:           d.IP,
			Method:       d.Method,
			Path:         d.Path,
			Status:       int32(d.Status),
			Agent:        d.Agent,
			UserId:       d.UserID,
			ErrorMessage: d.ErrorMessage,
			Body:         d.Body,
			Resp:         d.Resp,
		}
	}

	return &pb.ListLogsReply{
		Total: int32(count),
		List:  replyList,
	}, nil
}

// CleanSysOperationRecords 清理指定时间范围内的操作记录
func (s *SysLogsService) CleanLogs(ctx context.Context, req *pb.CleanLogsRequest) (*pb.CleanLogsReply, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	err := s.opRecordsCase.DeleteByTimeRange(ctx, req.StartTime, req.EndTime)
	if err != nil {
		s.log.Error(err)
		return nil, errors.InternalServer("OPERATION_RECORD_CLEAN_FAILED", "failed to clean operation records")
	}

	return &pb.CleanLogsReply{
		Deleted: 0, // Return actual count if available
	}, nil
}
