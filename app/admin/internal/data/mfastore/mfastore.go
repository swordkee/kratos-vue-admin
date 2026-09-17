// Package mfastore pkg/mfa 端口的 sys_users MFA 三列读写实现（gen 字段对象）。
//
// 独立叶子包（不并入 data/admin）：data/admin 各 repo 已 import biz/admin，
// 若端口实现放该包，biz provider 与 biz 测试均无法引用（import cycle）。
// 本包仅依赖 gen dao/model 与 pkg/mfa 端口，零 biz 依赖。
//
// 更新白名单采用 gen 字段对象 Select(q.MfaEnabled, ...) + struct Updates 形态
// （UNIFIED_CODE_STANDARDS §5.4「指定列（含写零值）」，先例=data/admin/
// sys_role_repo.go Update），禁止 map[string]any 与手写列名/字段名串；
// 三列全量覆写，禁用态写 Enabled=0/Secret=""/BoundAt=NULL，与旧 map 语义等价。
// 与 go-payAdmin 同名包同款（其 commit 6a756d6，「相同问题相同处理」）。
package mfastore

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/mfa"
)

// genStore mfa.MFAUserStore 的 gen DAO 实现
type genStore struct{ db *gorm.DB }

// New 构造 sys_users MFA 列读写 store（ProvideMFAService 注入 pkg/mfa）。
// 注意：gen 查询经 model.SysUsers（含 DeletedAt）自动追加 deleted_at IS NULL
// 软删过滤——软删用户按不存在处理（ErrNoUser），与 sys_user_repo 登录侧口径一致。
func New(db *gorm.DB) mfa.MFAUserStore { return &genStore{db: db} }

// Under 返回绑定 tx 会话的 store（Enable/Disable 与恢复码读写同事务原子提交）
func (s *genStore) Under(tx *gorm.DB) mfa.MFAUserStore { return &genStore{db: tx} }

// GetMFA 读取用户 MFA 三列；用户不存在（含软删）返回 found=false
func (s *genStore) GetMFA(ctx context.Context, userID int64) (mfa.UserMFA, bool, error) {
	q := dao.Use(s.db).SysUsers
	row, err := q.WithContext(ctx).
		Select(q.MfaEnabled, q.MfaSecret, q.MfaBoundAt).
		Where(q.ID.Eq(userID)).First()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return mfa.UserMFA{}, false, nil
	}
	if err != nil {
		return mfa.UserMFA{}, false, err
	}
	return mfa.UserMFA{Enabled: row.MfaEnabled != 0, Secret: row.MfaSecret, BoundAt: row.MfaBoundAt}, true, nil
}

// SetMFA 三列全量覆写（白名单强制写零值/NULL）；返回影响行数（0=用户不存在，
// 调用方归 ErrNoUser——与旧 map 更新 RowsAffected 判定等价）
func (s *genStore) SetMFA(ctx context.Context, userID int64, u mfa.UserMFA) (int64, error) {
	var enabled int32
	if u.Enabled {
		enabled = 1
	}
	q := dao.Use(s.db).SysUsers
	res, err := q.WithContext(ctx).Where(q.ID.Eq(userID)).
		Select(q.MfaEnabled, q.MfaSecret, q.MfaBoundAt).
		Updates(&model.SysUsers{MfaEnabled: enabled, MfaSecret: u.Secret, MfaBoundAt: u.BoundAt})
	return res.RowsAffected, err
}
