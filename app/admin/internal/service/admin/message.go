package admin

// 统一站内信服务（KVA 域 sys_message；TASK-03 批1 模板版，2026-09-23）。
//
// 形态照同目录 sys_logs/sys_mfa/dict 通用套路：req.Validate() → usecase → 结果转 pb。
// 信封：每个 Rsp 首两字段 code/msg（0=成功），HTTP 外层 EncoderResponse 会再包一层
// CommonReply{code,message,data}，前端解包 data 后读本层 code/msg 与载荷字段。
// 身份纪律不在本层复制：列表 uid 裁决与单条操作的 uid 注入全部收口在
// data/admin/message_repo.go（请求里的 uid 对单条操作物理不存在）。

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// MessageService 实现 v1.MessageServiceHTTPServer。
type MessageService struct {
	pb.UnimplementedMessageServiceServer
	uc  *admin.MessageUseCase
	log *logx.Logger
}

// NewMessageService 创建站内信服务。
func NewMessageService(uc *admin.MessageUseCase, logger *logx.Logger) *MessageService {
	return &MessageService{uc: uc, log: logger}
}

const messageSuccessMsg = "success"

// SendMessage 发送站内信（收件人任意 uid；sender 由 usecase 从 JWT claims 注入）。
func (s *MessageService) SendMessage(ctx context.Context, req *pb.SendMessageReq) (*pb.SendMessageRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if req.Uid <= 0 {
		return nil, errors.New(400, "INVALID_UID", "收件人 uid 不能为空")
	}
	if req.Type < admin.MessageTypeSystem || req.Type > admin.MessageTypeComplain {
		return nil, errors.New(400, "INVALID_MESSAGE_TYPE", "消息类型必须为 1系统/2订单/3结算/4投诉")
	}

	row, err := s.uc.SendMessage(ctx, &model.SysMessage{
		UID:     req.Uid,
		Type:    req.Type,
		Title:   req.Title,
		Content: req.Content,
		Scene:   req.Scene,
		Param:   req.Param,
	})
	if err != nil {
		s.log.Errorf(ctx, "SendMessage failed: %v", err)
		return nil, err
	}
	return &pb.SendMessageRsp{Code: 0, Msg: messageSuccessMsg, Id: row.ID}, nil
}

// ListMessages 分页列表（默认查自己，超管可传 uid 查他人；非超管查他人 403）。
func (s *MessageService) ListMessages(ctx context.Context, req *pb.ListMessagesReq) (*pb.ListMessagesRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	// type / is_read 均为可选包装：缺省=不过滤（is_read 用 -1 哨兵表达「全部」）
	condition := admin.MessageListCondition{Type: 0, IsRead: admin.MessageListAllRead}
	if v := req.GetType(); v != nil {
		condition.Type = v.GetValue()
	}
	if v := req.GetIsRead(); v != nil {
		condition.IsRead = v.GetValue()
	}

	rows, total, err := s.uc.ListPage(ctx, req.Uid, condition, req.Page, req.PageSize)
	if err != nil {
		s.log.Errorf(ctx, "ListMessages failed: %v", err)
		return nil, err
	}

	list := make([]*pb.Message, 0, len(rows))
	for _, d := range rows {
		list = append(list, convertSysMessage(d))
	}
	return &pb.ListMessagesRsp{Code: 0, Msg: messageSuccessMsg, Total: int32(total), List: list}, nil
}

// GetMessage 详情（收件人=JWT 注入；查不到 404，不泄露存在性）。
func (s *MessageService) GetMessage(ctx context.Context, req *pb.GetMessageReq) (*pb.GetMessageRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	row, err := s.uc.FindByID(ctx, req.Id)
	if err != nil {
		if isMessageNotFound(err) {
			return nil, errors.New(404, "MESSAGE_NOT_FOUND", "站内信不存在")
		}
		s.log.Errorf(ctx, "GetMessage failed: %v", err)
		return nil, err
	}
	return &pb.GetMessageRsp{Code: 0, Msg: messageSuccessMsg, Data: convertSysMessage(row)}, nil
}

// MarkRead 单条已读（不带 uid；重复已读幂等成功）。
func (s *MessageService) MarkRead(ctx context.Context, req *pb.MarkReadReq) (*pb.MarkReadRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	found, err := s.uc.MarkRead(ctx, req.Id)
	if err != nil {
		s.log.Errorf(ctx, "MarkRead failed: %v", err)
		return nil, err
	}
	if !found {
		return nil, errors.New(404, "MESSAGE_NOT_FOUND", "站内信不存在")
	}
	return &pb.MarkReadRsp{Code: 0, Msg: messageSuccessMsg}, nil
}

// MarkAllRead 全部已读（按本人 uid 批量）。
func (s *MessageService) MarkAllRead(ctx context.Context, req *pb.MarkAllReadReq) (*pb.MarkAllReadRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	updated, err := s.uc.MarkAllRead(ctx)
	if err != nil {
		s.log.Errorf(ctx, "MarkAllRead failed: %v", err)
		return nil, err
	}
	return &pb.MarkAllReadRsp{Code: 0, Msg: messageSuccessMsg, Updated: updated}, nil
}

// DeleteMessage 删除（软删；不带 uid）。
func (s *MessageService) DeleteMessage(ctx context.Context, req *pb.DeleteMessageReq) (*pb.DeleteMessageRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	found, err := s.uc.DeleteMessage(ctx, req.Id)
	if err != nil {
		s.log.Errorf(ctx, "DeleteMessage failed: %v", err)
		return nil, err
	}
	if !found {
		return nil, errors.New(404, "MESSAGE_NOT_FOUND", "站内信不存在")
	}
	return &pb.DeleteMessageRsp{Code: 0, Msg: messageSuccessMsg}, nil
}

// CountUnread 未读数（角标）。
func (s *MessageService) CountUnread(ctx context.Context, req *pb.CountUnreadReq) (*pb.CountUnreadRsp, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	count, err := s.uc.CountUnread(ctx)
	if err != nil {
		s.log.Errorf(ctx, "CountUnread failed: %v", err)
		return nil, err
	}
	return &pb.CountUnreadRsp{Code: 0, Msg: messageSuccessMsg, Count: count}, nil
}

// isMessageNotFound 仓储层「查不到（不存在或非本人）」判定。
func isMessageNotFound(err error) bool {
	return err != nil && errors.Is(err, gorm.ErrRecordNotFound)
}

// convertSysMessage model → pb（时间列统一输出毫秒时间戳，未发生的时间为 0）。
func convertSysMessage(d *model.SysMessage) *pb.Message {
	out := &pb.Message{
		Id:         d.ID,
		Uid:        d.UID,
		Type:       d.Type,
		Title:      d.Title,
		Content:    d.Content,
		Scene:      d.Scene,
		Param:      d.Param,
		SenderId:   d.SenderID,
		SenderName: d.SenderName,
		IsRead:     d.IsRead,
		CreatedAt:  d.CreatedAt.UnixMilli(),
		UpdatedAt:  d.UpdatedAt.UnixMilli(),
	}
	if d.ReadAt != nil && !d.ReadAt.IsZero() {
		out.ReadAt = d.ReadAt.UnixMilli()
	}
	if d.DeletedAt.Valid && !d.DeletedAt.Time.IsZero() {
		out.DeletedAt = d.DeletedAt.Time.UnixMilli()
	}
	return out
}

// 确保 time 被引用（ReadAt 为零值时保持 0 语义，避免误删 time 依赖）。
var _ = time.Time{}
