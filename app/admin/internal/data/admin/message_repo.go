package admin

// 站内信仓储（KVA 域 sys_message；TASK-03 批1 模板版，2026-09-23）。
//
// 形态照 data/admin 同包 sys_logs/sys_dict 通用套路（分页+Count、gen 字段对象查询），
// 叠加三条纪律：
//  1. 软删：所有读路径显式 `deleted_at IS NULL`（模型 gorm.DeletedAt 自动条件之外
//     再钉一次，语义显式化，防 regen/查询体替换时静默丢条件）；
//  2. 写路径一律 UpdateSimple（字段级表达式），禁 Updates(map)/Updates(struct)——
//     零值字段会被 GORM 跳过（ADM-03/MKT-B4 已修同型缺陷）；
//  3. 身份 fail-closed：uid 只从 JWT claims 解析（见 resolveMessageQueryUID），
//     已读/删除用 `id AND uid` 复合条件，重复已读幂等成功（0 行且已读=success）。

import (
	"context"
	stderrors "errors"
	"time"

	"github.com/go-kratos/kratos/v3/errors"
	"gorm.io/gorm"

	admin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

type messageRepo struct {
	query *dao.Query
	log   *logx.Logger
}

// NewMessageRepo 构造站内信仓储（默认组 gen Query，库位随 data.database.source）。
func NewMessageRepo(query *dao.Query, logger *logx.Logger) admin.MessageRepo {
	return &messageRepo{query: query, log: logger}
}

// resolveMessageQueryUID 计算站内信查询/操作的目标收件人 uid（fail-closed，
// 文案风格对齐 exAdmin data/query.resolveQueryMemberScope）：
//   - 无 JWT 身份上下文：401，不返回空集伪装成功；
//   - requestedUID<=0 或等于本人：本人 uid；
//   - 超管（role_key=admin）查他人：放行 requestedUID；
//   - 非超管查他人：403（请求侧 uid 属不可信输入，不参与过滤条件组装）。
func resolveMessageQueryUID(ctx context.Context, log *logx.Logger, operation string, requestedUID int32) (int32, error) {
	claims, claimErr := authz.FromContext(ctx)
	if claimErr != nil || claims == nil {
		if log != nil {
			log.Warnf(ctx, "message scope: missing operator identity, rejecting query (fail-closed): op=%s", operation)
		}
		return 0, errors.New(401, "IDENTITY_REQUIRED", "缺少操作员身份，禁止查询站内信")
	}
	ownUID := int32(claims.UserID)
	if requestedUID <= 0 || requestedUID == ownUID {
		return ownUID, nil
	}
	if authz.IsSuperAdmin(claims.RoleKey) {
		if log != nil {
			log.Infof(ctx, "message scope: super admin queries uid=%d: op=%s", requestedUID, operation)
		}
		return requestedUID, nil
	}
	if log != nil {
		log.Warnf(ctx, "message scope: operator %d rejected querying uid=%d (non-super-admin, fail-closed): op=%s",
			claims.UserID, requestedUID, operation)
	}
	return 0, errors.New(403, "MESSAGE_QUERY_FORBIDDEN", "仅超级管理员可查询他人站内信（当前操作员无权限）")
}

// ownerUID 单条操作（详情/已读/删除/未读数）的目标收件人：一律本人（请求不带 uid）。
func ownerUID(ctx context.Context, log *logx.Logger, operation string) (int32, error) {
	return resolveMessageQueryUID(ctx, log, operation, 0)
}

// Create 落一条站内信（收件人/类型/标题等由 usecase 组装，sender 由 biz 从 claims 注入）。
func (r *messageRepo) Create(ctx context.Context, g *model.SysMessage) error {
	q := r.query.SysMessage
	return q.WithContext(ctx).Create(g)
}

// ListPage 分页查询（软删过滤 + type/is_read 过滤；total 与列表同源）。
func (r *messageRepo) ListPage(ctx context.Context, queryUID int32, condition admin.MessageListCondition, page, size int32) ([]*model.SysMessage, int64, error) {
	uid, err := resolveMessageQueryUID(ctx, r.log, "message.list", queryUID)
	if err != nil {
		return nil, 0, err
	}

	q := r.query.SysMessage
	db := q.WithContext(ctx).Where(q.UID.Eq(uid), q.DeletedAt.IsNull())
	if condition.Type > 0 {
		db = db.Where(q.Type.Eq(condition.Type))
	}
	// IsRead<0（哨兵 -1）=全部；0/1=按已读状态过滤
	if condition.IsRead >= 0 {
		db = db.Where(q.IsRead.Eq(condition.IsRead))
	}

	count, err := db.Count()
	if err != nil {
		return nil, 0, err
	}
	if count == 0 {
		return []*model.SysMessage{}, 0, nil
	}

	limit, offset := convertPageSize(page, size)
	rows, err := db.Limit(limit).Offset(offset).Order(q.ID.Desc()).Find()
	if err != nil {
		return nil, 0, err
	}
	return rows, count, nil
}

// FindByID 按 id+uid 复合条件取详情（查不到按「不存在」返回，不泄露存在性）。
func (r *messageRepo) FindByID(ctx context.Context, id int64) (*model.SysMessage, error) {
	uid, err := ownerUID(ctx, r.log, "message.get")
	if err != nil {
		return nil, err
	}
	q := r.query.SysMessage
	return q.WithContext(ctx).
		Where(q.ID.Eq(id), q.UID.Eq(uid), q.DeletedAt.IsNull()).
		First()
}

// MarkRead 标记已读：`WHERE id=? AND uid=? AND deleted_at IS NULL AND is_read=0`。
// 返回 found=false 表示不存在或非本人；重复已读（0 行且已读）返回 true（幂等 success）。
func (r *messageRepo) MarkRead(ctx context.Context, id int64) (bool, error) {
	uid, err := ownerUID(ctx, r.log, "message.markRead")
	if err != nil {
		return false, err
	}

	q := r.query.SysMessage
	now := time.Now()
	result, err := q.WithContext(ctx).
		Where(q.ID.Eq(id), q.UID.Eq(uid), q.DeletedAt.IsNull(), q.IsRead.Eq(admin.MessageUnread)).
		UpdateSimple(
			q.IsRead.Value(admin.MessageRead),
			q.ReadAt.Value(now),
			q.UpdatedAt.Value(now),
		)
	if err != nil {
		return false, err
	}
	if result.RowsAffected > 0 {
		return true, nil
	}

	// 0 行：已读（幂等 success）/ 不存在或非本人（false，不泄露存在性）
	row, findErr := q.WithContext(ctx).
		Where(q.ID.Eq(id), q.UID.Eq(uid), q.DeletedAt.IsNull()).
		First()
	if findErr != nil {
		if stderrors.Is(findErr, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, findErr
	}
	return row.IsRead == admin.MessageRead, nil
}

// MarkAllRead 按 uid 批量置已读，返回本次更新行数。
func (r *messageRepo) MarkAllRead(ctx context.Context) (int64, error) {
	uid, err := ownerUID(ctx, r.log, "message.markAllRead")
	if err != nil {
		return 0, err
	}

	q := r.query.SysMessage
	now := time.Now()
	result, err := q.WithContext(ctx).
		Where(q.UID.Eq(uid), q.DeletedAt.IsNull(), q.IsRead.Eq(admin.MessageUnread)).
		UpdateSimple(
			q.IsRead.Value(admin.MessageRead),
			q.ReadAt.Value(now),
			q.UpdatedAt.Value(now),
		)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected, nil
}

// SoftDelete 软删：`WHERE id=? AND uid=?`，由模型 gorm.DeletedAt 走 UPDATE deleted_at。
// 返回 found=false 表示不存在或非本人。
func (r *messageRepo) SoftDelete(ctx context.Context, id int64) (bool, error) {
	uid, err := ownerUID(ctx, r.log, "message.delete")
	if err != nil {
		return false, err
	}

	q := r.query.SysMessage
	result, err := q.WithContext(ctx).
		Where(q.ID.Eq(id), q.UID.Eq(uid), q.DeletedAt.IsNull()).
		Delete()
	if err != nil {
		return false, err
	}
	return result.RowsAffected > 0, nil
}

// CountUnread 未读数（uid + is_read=0 + 未删）。
func (r *messageRepo) CountUnread(ctx context.Context) (int64, error) {
	uid, err := ownerUID(ctx, r.log, "message.countUnread")
	if err != nil {
		return 0, err
	}

	q := r.query.SysMessage
	return q.WithContext(ctx).
		Where(q.UID.Eq(uid), q.DeletedAt.IsNull(), q.IsRead.Eq(admin.MessageUnread)).
		Count()
}
