/*
 * message_repo_test.go - 统一站内信仓储测试（内存 SQLite，TASK-03 批1 模板版，2026-09-23）
 *
 * 覆盖口径（对应任务验收项）：
 *   1. 分页 + 软删过滤（deleted_at IS NULL，total 与列表同源）；
 *   2. uid 身份纪律：无身份 401 / 非超管查他人 403 / 超管可查他人 /
 *      单条操作（详情/已读/删除/未读数）请求侧无 uid，只认 JWT claims（他人消息不可达）；
 *   3. 已读 = id+uid 复合条件 + 重复已读幂等成功（0 行且已读=success）；
 *   4. MarkAllRead 按 uid 批量且不越界；
 *   5. CountUnread（uid + is_read=0 + 未删）；
 *   6. type / is_read 过滤（is_read=-1 或缺省=全部）；
 *   7. 可空列（scene/param/sender_id/sender_name/read_at 为 NULL）读路径不炸。
 */
package admin

import (
	"context"
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"gorm.io/gorm"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	adminbiz "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"

	"google.golang.org/protobuf/proto"
)

type messageFixture struct {
	repo adminbiz.MessageRepo
	q    *dao.Query
	db   *gorm.DB
}

// newMessageRepoFixture 建内存库并挂 sys_message 表。
func newMessageRepoFixture(t *testing.T) *messageFixture {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(10000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysMessage{}))
	q := dao.Use(db)
	return &messageFixture{repo: NewMessageRepo(q, logx.NewLogger(nil)), q: q, db: db}
}

// 三类操作员身份 + 无身份
func messageSuperCtx() context.Context {
	return authz.NewContext(context.Background(), &authz.TokenClaims{UserID: 1, RoleKey: authz.SuperAdminRoleKey})
}

func messageCtx(uid int64) context.Context {
	return authz.NewContext(context.Background(), &authz.TokenClaims{
		UserID:   uid,
		RoleKey:  "manage",
		Nickname: fmt.Sprintf("op%d", uid),
	})
}

func messageNoIdentityCtx() context.Context {
	return context.Background()
}

// seedMessage 写一条站内信（uid/type/isRead 由调用方指定）。
func seedMessage(t *testing.T, q *dao.Query, uid, messageType, isRead int32) *model.SysMessage {
	t.Helper()
	row := &model.SysMessage{
		UID:     uid,
		Type:    messageType,
		Title:   fmt.Sprintf("消息 uid=%d type=%d read=%d", uid, messageType, isRead),
		Content: "正文",
		IsRead:  isRead,
	}
	require.NoError(t, q.SysMessage.WithContext(context.Background()).Create(row))
	return row
}

// assertMessageKratosCode 断言错误为指定 code 的 kratos 错误。
func assertMessageKratosCode(t *testing.T, err error, code int32) {
	t.Helper()
	require.Error(t, err)
	var ke *kerrors.Error
	require.True(t, kerrors.As(err, &ke), "expected kratos error, got %T: %v", err, err)
	assert.Equal(t, code, ke.Code, "unexpected error code: %v", err)
}

// 1. 分页 + 软删过滤 + 跨 uid 隔离
func TestMessageRepo_ListPage_PagingAndSoftDelete(t *testing.T) {
	f := newMessageRepoFixture(t)
	ctx := messageCtx(7)

	var seeded []*model.SysMessage
	for i := 0; i < 12; i++ {
		seeded = append(seeded, seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread))
	}
	other := seedMessage(t, f.q, 8, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)

	// 软删最后一条：列表 total 应同步减一，但行仍在库中
	deleted := seeded[len(seeded)-1]
	found, err := f.repo.SoftDelete(ctx, deleted.ID)
	require.NoError(t, err)
	require.True(t, found)
	var persisted int64
	require.NoError(t, f.db.Unscoped().Model(&model.SysMessage{}).Where("id = ?", deleted.ID).Count(&persisted).Error)
	require.EqualValues(t, 1, persisted, "软删必须是 UPDATE deleted_at，不是物理删除")

	rows, total, err := f.repo.ListPage(ctx, 0, adminbiz.MessageListCondition{IsRead: adminbiz.MessageListAllRead}, 1, 5)
	require.NoError(t, err)
	assert.EqualValues(t, 11, total, "软删行不得计入 total")
	assert.Len(t, rows, 5)
	for _, r := range rows {
		assert.EqualValues(t, 7, r.UID, "列表不得混入他人消息")
		assert.NotEqual(t, deleted.ID, r.ID, "软删行不得出现在列表")
	}

	rows3, total3, err := f.repo.ListPage(ctx, 0, adminbiz.MessageListCondition{IsRead: adminbiz.MessageListAllRead}, 3, 5)
	require.NoError(t, err)
	assert.EqualValues(t, 11, total3)
	assert.Len(t, rows3, 1, "第 3 页仅剩 1 条（11-5-5）")

	// 排序：按 id 倒序
	if len(rows) > 1 {
		assert.Greater(t, rows[0].ID, rows[1].ID)
	}

	// 另一 uid 的消息只在其本人视图出现
	otherRows, otherTotal, err := f.repo.ListPage(messageCtx(8), 0, adminbiz.MessageListCondition{IsRead: adminbiz.MessageListAllRead}, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, otherTotal)
	require.Len(t, otherRows, 1)
	assert.Equal(t, other.ID, otherRows[0].ID)
}

// 2. type / is_read 过滤（is_read=-1 或缺省语义=全部由 service 传入 -1）
func TestMessageRepo_ListPage_TypeAndReadFilters(t *testing.T) {
	f := newMessageRepoFixture(t)
	ctx := messageCtx(7)

	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeOrder, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeOrder, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageRead)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSettle, adminbiz.MessageRead)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeComplain, adminbiz.MessageUnread)

	cases := []struct {
		name      string
		condition adminbiz.MessageListCondition
		want      int64
	}{
		{"全部（不过滤类型 + 已读哨兵-1）", adminbiz.MessageListCondition{Type: 0, IsRead: -1}, 7},
		{"按类型=系统", adminbiz.MessageListCondition{Type: adminbiz.MessageTypeSystem, IsRead: -1}, 3},
		{"按类型=订单", adminbiz.MessageListCondition{Type: adminbiz.MessageTypeOrder, IsRead: -1}, 2},
		{"按类型=投诉", adminbiz.MessageListCondition{Type: adminbiz.MessageTypeComplain, IsRead: -1}, 1},
		{"按类型=不存在的类型5", adminbiz.MessageListCondition{Type: 5, IsRead: -1}, 0},
		{"按已读=未读", adminbiz.MessageListCondition{Type: 0, IsRead: adminbiz.MessageUnread}, 5},
		{"按已读=已读", adminbiz.MessageListCondition{Type: 0, IsRead: adminbiz.MessageRead}, 2},
		{"类型+已读 组合", adminbiz.MessageListCondition{Type: adminbiz.MessageTypeOrder, IsRead: adminbiz.MessageUnread}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rows, total, err := f.repo.ListPage(ctx, 0, c.condition, 1, 100)
			require.NoError(t, err)
			assert.Equal(t, c.want, total)
			assert.Len(t, rows, int(c.want))
		})
	}
}

// 3. 无身份 fail-closed：全部入口 401（发生在 SQL 之前）
func TestMessageRepo_IdentityFailClosed(t *testing.T) {
	f := newMessageRepoFixture(t)
	row := seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	ctx := messageNoIdentityCtx()

	_, _, err := f.repo.ListPage(ctx, 0, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
	assertMessageKratosCode(t, err, 401)

	_, err = f.repo.FindByID(ctx, row.ID)
	assertMessageKratosCode(t, err, 401)

	_, err = f.repo.MarkRead(ctx, row.ID)
	assertMessageKratosCode(t, err, 401)

	_, err = f.repo.MarkAllRead(ctx)
	assertMessageKratosCode(t, err, 401)

	_, err = f.repo.SoftDelete(ctx, row.ID)
	assertMessageKratosCode(t, err, 401)

	_, err = f.repo.CountUnread(ctx)
	assertMessageKratosCode(t, err, 401)
}

// 4. uid 越权反例：非超管查他人 403；超管可查他人；单条操作请求 uid 不存在（只认 claims）
func TestMessageRepo_QueryOthers_Overreach(t *testing.T) {
	f := newMessageRepoFixture(t)

	// 目标用户 100 有 2 条，另一用户 200 有 1 条
	seedMessage(t, f.q, 100, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 100, adminbiz.MessageTypeOrder, adminbiz.MessageUnread)
	target := seedMessage(t, f.q, 200, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)

	t.Run("非超管查他人 403", func(t *testing.T) {
		_, _, err := f.repo.ListPage(messageCtx(7), 100, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
		assertMessageKratosCode(t, err, 403)

		// 非超管查另一个他人同样 403（不是只挡一个目标）
		_, _, err = f.repo.ListPage(messageCtx(7), 200, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
		assertMessageKratosCode(t, err, 403)
	})

	t.Run("超管可查他人", func(t *testing.T) {
		rows, total, err := f.repo.ListPage(messageSuperCtx(), 100, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
		require.NoError(t, err)
		assert.EqualValues(t, 2, total)
		require.Len(t, rows, 2)
		for _, r := range rows {
			assert.EqualValues(t, 100, r.UID)
		}
	})

	t.Run("查自己（uid=本人）放行", func(t *testing.T) {
		rows, total, err := f.repo.ListPage(messageCtx(7), 7, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
		require.NoError(t, err)
		assert.EqualValues(t, 1, total)
		assert.Len(t, rows, 1)
	})

	t.Run("请求 uid 被忽略：单条操作只认 claims，他人消息不可达", func(t *testing.T) {
		// 详情：以 uid=7 的身份查 uid=200 的消息 → 按「不存在」返回（不泄露存在性）
		_, err := f.repo.FindByID(messageCtx(7), target.ID)
		require.Error(t, err)
		require.True(t, stderrors.Is(err, gorm.ErrRecordNotFound), "他人消息应按不存在返回，got %v", err)

		// 已读：复合条件 id+uid 不命中 → found=false，且目标行仍为未读
		found, err := f.repo.MarkRead(messageCtx(7), target.ID)
		require.NoError(t, err)
		assert.False(t, found, "他人消息不可被标记已读")

		// 删除：同上，目标行不得被删
		found, err = f.repo.SoftDelete(messageCtx(7), target.ID)
		require.NoError(t, err)
		assert.False(t, found)

		// 未读数：uid=7 视角只统计自己的未读（目标行未读也不计入）
		count, err := f.repo.CountUnread(messageCtx(7))
		require.NoError(t, err)
		assert.EqualValues(t, 1, count, "未读数按 claims uid 统计，不串他人")

		// 超管也不能用单条方法「替他人改」——单条操作固定为本人 uid（无 uid 入参）
		found, err = f.repo.MarkRead(messageSuperCtx(), target.ID)
		require.NoError(t, err)
		assert.False(t, found, "单条操作 uid 恒取 claims（超管 UserID=1），请求 uid 物理不存在")
	})
}

// 5. 已读 = 复合条件；重复已读幂等成功（0 行且已读 → success，read_at 不再改写）
func TestMessageRepo_MarkRead_CompositeAndIdempotent(t *testing.T) {
	f := newMessageRepoFixture(t)
	row := seedMessage(t, f.q, 100, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)

	// 复合条件：非收件人不可读
	found, err := f.repo.MarkRead(messageCtx(7), row.ID)
	require.NoError(t, err)
	require.False(t, found)
	_, err = f.repo.FindByID(messageCtx(100), row.ID)
	require.NoError(t, err)
	assert.EqualValues(t, adminbiz.MessageUnread, mustReload(t, f, row.ID).IsRead, "他人调用不得改动状态")

	// 收件人首读成功
	found, err = f.repo.MarkRead(messageCtx(100), row.ID)
	require.NoError(t, err)
	assert.True(t, found)
	first := mustReload(t, f, row.ID)
	assert.EqualValues(t, adminbiz.MessageRead, first.IsRead)
	require.NotNil(t, first.ReadAt, "已读需写 read_at")
	assert.False(t, first.ReadAt.IsZero(), "已读需写 read_at")

	// 重复已读：幂等成功且 read_at 不被覆盖
	found, err = f.repo.MarkRead(messageCtx(100), row.ID)
	require.NoError(t, err)
	assert.True(t, found, "重复已读必须幂等 success（0 行且已读）")
	second := mustReload(t, f, row.ID)
	require.NotNil(t, second.ReadAt)
	assert.True(t, first.ReadAt.Equal(*second.ReadAt), "幂等调用不得改写 read_at")

	// 不存在的 id → found=false（不报错，service 层映射 404）
	found, err = f.repo.MarkRead(messageCtx(100), 999999)
	require.NoError(t, err)
	assert.False(t, found)
}

// 6. MarkAllRead 按 uid 批量，且不越界到他人
func TestMessageRepo_MarkAllRead(t *testing.T) {
	f := newMessageRepoFixture(t)

	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeOrder, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSettle, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageRead)
	seedMessage(t, f.q, 100, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 100, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)

	updated, err := f.repo.MarkAllRead(messageCtx(7))
	require.NoError(t, err)
	assert.EqualValues(t, 3, updated, "只把本人未读置为已读（已读那条不重复计数）")

	// 幂等：再跑一次为 0
	updated, err = f.repo.MarkAllRead(messageCtx(7))
	require.NoError(t, err)
	assert.EqualValues(t, 0, updated)

	// 他人未读不受影响
	count, err := f.repo.CountUnread(messageCtx(100))
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)

	// 本人已清零
	count, err = f.repo.CountUnread(messageCtx(7))
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)
}

// 7. CountUnread：uid + is_read=0 + 未删；他人消息不计入
func TestMessageRepo_CountUnread(t *testing.T) {
	f := newMessageRepoFixture(t)
	ctx := messageCtx(7)

	seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeOrder, adminbiz.MessageUnread)
	seedMessage(t, f.q, 7, adminbiz.MessageTypeSettle, adminbiz.MessageRead)
	// 他人未读 + 本人已软删的未读：都不计入
	seedMessage(t, f.q, 100, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	doomed := seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)
	found, err := f.repo.SoftDelete(ctx, doomed.ID)
	require.NoError(t, err)
	require.True(t, found)

	count, err := f.repo.CountUnread(ctx)
	require.NoError(t, err)
	assert.EqualValues(t, 2, count)
}

// 8. 删除=软删 + 复合条件；删后列表/详情/未读数全部不可见
func TestMessageRepo_SoftDelete(t *testing.T) {
	f := newMessageRepoFixture(t)
	row := seedMessage(t, f.q, 7, adminbiz.MessageTypeSystem, adminbiz.MessageUnread)

	// 他人删除：不生效
	found, err := f.repo.SoftDelete(messageCtx(100), row.ID)
	require.NoError(t, err)
	require.False(t, found)
	assert.EqualValues(t, adminbiz.MessageUnread, mustReload(t, f, row.ID).IsRead)
	assert.False(t, mustDeletedAt(t, f, row.ID).Valid, "他人删除不得写 deleted_at")

	// 本人删除：软删（行保留，deleted_at 置位）
	found, err = f.repo.SoftDelete(messageCtx(7), row.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, mustDeletedAt(t, f, row.ID).Valid, "删除必须走软删 UPDATE deleted_at")

	// 列表 / 详情 / 未读数全部不可见
	_, total, err := f.repo.ListPage(messageCtx(7), 0, adminbiz.MessageListCondition{IsRead: -1}, 1, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total)

	_, err = f.repo.FindByID(messageCtx(7), row.ID)
	require.True(t, stderrors.Is(err, gorm.ErrRecordNotFound))

	found, err = f.repo.MarkRead(messageCtx(7), row.ID)
	require.NoError(t, err)
	assert.False(t, found, "已软删的消息不可再标记已读")

	count, err := f.repo.CountUnread(messageCtx(7))
	require.NoError(t, err)
	assert.EqualValues(t, 0, count)

	// 重复删除幂等：found=false（行已不可见）
	found, err = f.repo.SoftDelete(messageCtx(7), row.ID)
	require.NoError(t, err)
	assert.False(t, found)
}

// 9. 可空列（scene/param/sender_id/sender_name/read_at = NULL）读路径不炸
func TestMessageRepo_NullableColumnsReadable(t *testing.T) {
	f := newMessageRepoFixture(t)

	require.NoError(t, f.db.Exec(`
		INSERT INTO sys_message (uid, type, title, content, scene, param, sender_id, sender_name,
		                         is_read, read_at, deleted_at, created_at, updated_at)
		VALUES (7, 1, '系统消息', '正文', NULL, NULL, NULL, NULL, 0, NULL, NULL, datetime('now'), datetime('now'))
	`).Error)

	rows, total, err := f.repo.ListPage(messageCtx(7), 0, adminbiz.MessageListCondition{IsRead: adminbiz.MessageListAllRead}, 1, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, "系统消息", rows[0].Title)

	count, err := f.repo.CountUnread(messageCtx(7))
	require.NoError(t, err)
	assert.EqualValues(t, 1, count)

	found, err := f.repo.MarkRead(messageCtx(7), rows[0].ID)
	require.NoError(t, err)
	assert.True(t, found)
}

// 10. MarkRead/Delete/Get/CountUnread 请求不带 uid：proto 层未知 uid 字段被丢弃
// （服务端 JWT 注入，客户端即使塞 uid 也到不了业务层）
func TestMessageRepo_RequestCarriesNoUIDField(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		msg  proto.Message
	}{
		{"MarkRead", `{"id":1,"uid":999}`, &pb.MarkReadReq{}},
		{"DeleteMessage", `{"id":1,"uid":999}`, &pb.DeleteMessageReq{}},
		{"GetMessage", `{"id":1,"uid":999}`, &pb.GetMessageReq{}},
		{"CountUnread", `{"uid":999}`, &pb.CountUnreadReq{}},
		{"MarkAllRead", `{"uid":999}`, &pb.MarkAllReadReq{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			require.NoError(t,
				protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal([]byte(c.raw), c.msg),
				"未知 uid 字段必须被 DiscardUnknown 丢弃（不能因多带 uid 报错）")
			require.Nil(t, c.msg.ProtoReflect().Descriptor().Fields().ByName("uid"),
				"请求消息不得声明 uid 字段：收件人由服务端 JWT claims 注入")
		})
	}
}

// mustReload 直查库行（绕开 repo 的身份过滤）。
func mustReload(t *testing.T, f *messageFixture, id int64) *model.SysMessage {
	t.Helper()
	var row *model.SysMessage
	require.NoError(t, f.db.Unscoped().Where("id = ?", id).First(&row).Error)
	return row
}

// mustDeletedAt 取软删标记。
func mustDeletedAt(t *testing.T, f *messageFixture, id int64) gorm.DeletedAt {
	t.Helper()
	var row model.SysMessage
	require.NoError(t, f.db.Unscoped().Where("id = ?", id).First(&row).Error)
	return row.DeletedAt
}
