package admin

// 站内信（统一站内信，KVA 域 sys_message；TASK-03 批1 模板版，2026-09-23）
//
// 分层照 sys_logs/sys_dict 同型：repo 接口 + usecase 薄封装。
// 身份纪律（全部收口在 repo 层，见 data/admin/message_repo.go 的
// resolveMessageQueryUID，fail-closed 文案风格对齐 exAdmin data/query.resolveQueryMemberScope）：
//   - 单条读/已读/删除/未读数：收件人一律取 JWT claims 的 UserID，请求不带 uid；
//   - 列表：默认查自己，超管可传 uid 查他人，非超管查他人 403、无身份 401；
//   - SendMessage：uid=收件人（可为任意用户），sender_id/sender_name 由本层从
//     claims 注入（同 CreateDictData 的 CreateBy 注入口径）。

import (
	"context"

	"github.com/go-kratos/kratos/v3/errors"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// 站内信类型（sys_message.type）
const (
	MessageTypeSystem   int32 = 1 // 系统
	MessageTypeOrder    int32 = 2 // 订单
	MessageTypeSettle   int32 = 3 // 结算
	MessageTypeComplain int32 = 4 // 投诉
)

// 站内信已读状态（sys_message.is_read）
const (
	MessageUnread int32 = 0 // 未读
	MessageRead   int32 = 1 // 已读
)

// MessageListAllRead 列表「已读状态不过滤」哨兵（-1=全部，0/1=按状态过滤）
const MessageListAllRead int32 = -1

// MessageListCondition 站内信列表过滤条件。
// Type=0 表示类型不过滤；IsRead 取 MessageListAllRead(-1)=全部、0=未读、1=已读
// （调用方必须显式赋值，零值 0 语义是「未读」而非「全部」）。
type MessageListCondition struct {
	Type   int32
	IsRead int32
}

// MessageRepo 站内信仓储接口。
// queryUID 仅列表使用（0=查自己，>0=目标 uid，由 repo 内按超管/本人裁决）；
// 其余方法不带 uid 参数——请求侧 uid 物理不存在，只能来自 JWT claims。
type MessageRepo interface {
	Create(ctx context.Context, g *model.SysMessage) error
	ListPage(ctx context.Context, queryUID int32, condition MessageListCondition, page, size int32) ([]*model.SysMessage, int64, error)
	FindByID(ctx context.Context, id int64) (*model.SysMessage, error)
	MarkRead(ctx context.Context, id int64) (bool, error)
	MarkAllRead(ctx context.Context) (int64, error)
	SoftDelete(ctx context.Context, id int64) (bool, error)
	CountUnread(ctx context.Context) (int64, error)
}

// MessageUseCase 站内信用例。
type MessageUseCase struct {
	repo MessageRepo
	log  *logx.Logger
}

// NewMessageUseCase 创建站内信用例。
func NewMessageUseCase(repo MessageRepo, logger *logx.Logger) *MessageUseCase {
	return &MessageUseCase{repo: repo, log: logger}
}

// SendMessage 发送站内信：sender_id/sender_name 由 JWT claims 注入（无身份 401），
// 收件人 uid 由调用方指定（任意 uid，管理面发信允许发给任意用户）。
func (uc *MessageUseCase) SendMessage(ctx context.Context, g *model.SysMessage) (*model.SysMessage, error) {
	claims, err := authz.FromContext(ctx)
	if err != nil || claims == nil {
		return nil, errors.New(401, "IDENTITY_REQUIRED", "缺少操作员身份，禁止发送站内信")
	}
	g.SenderID = int32(claims.UserID)
	g.SenderName = claims.Nickname
	if err := uc.repo.Create(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// ListPage 分页查询站内信（uid 裁决在 repo：0=自己，超管可查他人）。
func (uc *MessageUseCase) ListPage(ctx context.Context, uid int32, condition MessageListCondition, page, size int32) ([]*model.SysMessage, int64, error) {
	return uc.repo.ListPage(ctx, uid, condition, page, size)
}

// FindByID 查询单条站内信详情（收件人=JWT claims）。
func (uc *MessageUseCase) FindByID(ctx context.Context, id int64) (*model.SysMessage, error) {
	return uc.repo.FindByID(ctx, id)
}

// MarkRead 标记单条已读。返回 false 表示不存在或非本人（不泄露存在性）。
func (uc *MessageUseCase) MarkRead(ctx context.Context, id int64) (bool, error) {
	return uc.repo.MarkRead(ctx, id)
}

// MarkAllRead 全部已读，返回本次置为已读的条数。
func (uc *MessageUseCase) MarkAllRead(ctx context.Context) (int64, error) {
	return uc.repo.MarkAllRead(ctx)
}

// DeleteMessage 删除（软删）单条站内信。返回 false 表示不存在或非本人。
func (uc *MessageUseCase) DeleteMessage(ctx context.Context, id int64) (bool, error) {
	return uc.repo.SoftDelete(ctx, id)
}

// CountUnread 未读数（角标）。
func (uc *MessageUseCase) CountUnread(ctx context.Context) (int64, error) {
	return uc.repo.CountUnread(ctx)
}
