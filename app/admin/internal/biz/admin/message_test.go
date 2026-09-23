/*
 * message_test.go - 统一站内信用例测试（TASK-03 批1 模板版，2026-09-23）
 *
 * 覆盖：SendMessage 的 sender_id/sender_name 由 JWT claims 注入（无身份 401）、
 * 收件人 uid 原样透传（管理面可发任意 uid），以及列表 uid 请求值原样下传
 * （越权裁决在 repo 层，见 data/admin/message_repo_test.go）。
 */
package admin

import (
	"context"
	"testing"

	kerrors "github.com/go-kratos/kratos/v3/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
	"github.com/swordkee/kratos-vue-admin/pkg/logx"
)

// newMessageTestLogger 测试用 logger（模板 biz/admin 此前无测试文件，
// 故 helper 以 message 前缀命名，避免与后续移植的其它 *_test.go 撞名）。
func newMessageTestLogger() *logx.Logger {
	return logx.NewLogger(nil)
}

// fakeMessageRepo 只记录落库入参的仓储桩（未用方法返回零值）。
type fakeMessageRepo struct {
	created *model.SysMessage
	// lastListUID / lastListCond 记录列表请求下传的 uid 与过滤条件
	lastListUID  int32
	lastListCond MessageListCondition
	lastListPage int32
	lastListSize int32
	listRows     []*model.SysMessage
	listTotal    int64
}

func (f *fakeMessageRepo) Create(_ context.Context, g *model.SysMessage) error {
	f.created = g
	return nil
}

func (f *fakeMessageRepo) ListPage(_ context.Context, queryUID int32, condition MessageListCondition, page, size int32) ([]*model.SysMessage, int64, error) {
	f.lastListUID = queryUID
	f.lastListCond = condition
	f.lastListPage = page
	f.lastListSize = size
	return f.listRows, f.listTotal, nil
}

func (f *fakeMessageRepo) FindByID(context.Context, int64) (*model.SysMessage, error) {
	return nil, nil
}

func (f *fakeMessageRepo) MarkRead(context.Context, int64) (bool, error) { return true, nil }

func (f *fakeMessageRepo) MarkAllRead(context.Context) (int64, error) { return 0, nil }

func (f *fakeMessageRepo) SoftDelete(context.Context, int64) (bool, error) { return true, nil }

func (f *fakeMessageRepo) CountUnread(context.Context) (int64, error) { return 0, nil }

// messageSenderCtx 带发件人身份的上下文。
func messageSenderCtx(userID int64, nickname string) context.Context {
	return authz.NewContext(context.Background(), &authz.TokenClaims{
		UserID:   userID,
		RoleKey:  "manage",
		Nickname: nickname,
	})
}

// SendMessage：sender_id/sender_name=claims；uid=收件人原样保留（可为任意用户）。
func TestMessageUseCase_SendMessage_SenderInjectedFromClaims(t *testing.T) {
	repo := &fakeMessageRepo{}
	uc := NewMessageUseCase(repo, newMessageTestLogger())

	row, err := uc.SendMessage(messageSenderCtx(42, "操作员A"), &model.SysMessage{
		UID:     7,
		Type:    MessageTypeSystem,
		Title:   "标题",
		Content: "正文",
	})
	require.NoError(t, err)
	require.NotNil(t, repo.created)

	assert.EqualValues(t, 42, repo.created.SenderID, "sender_id 必须来自 JWT claims")
	assert.Equal(t, "操作员A", repo.created.SenderName, "sender_name 必须来自 JWT claims")
	assert.EqualValues(t, 7, repo.created.UID, "收件人 uid 原样保留（SendMessage 允许任意 uid）")
	assert.EqualValues(t, 7, row.UID)
	assert.EqualValues(t, MessageTypeSystem, repo.created.Type)
}

// SendMessage：无身份 fail-closed 401，且不落库。
func TestMessageUseCase_SendMessage_NoIdentityRejected(t *testing.T) {
	repo := &fakeMessageRepo{}
	uc := NewMessageUseCase(repo, newMessageTestLogger())

	_, err := uc.SendMessage(context.Background(), &model.SysMessage{UID: 7, Type: MessageTypeSystem, Title: "标题"})
	require.Error(t, err)
	var ke *kerrors.Error
	require.True(t, kerrors.As(err, &ke))
	assert.EqualValues(t, 401, ke.Code)
	assert.Nil(t, repo.created, "无身份时不得写库")
}

// ListPage：请求 uid/过滤条件/分页参数原样下传（越权裁决不在 biz 层放水）。
func TestMessageUseCase_ListPage_ForwardsUIDAndCondition(t *testing.T) {
	repo := &fakeMessageRepo{listTotal: 3, listRows: []*model.SysMessage{{ID: 1, UID: 100}}}
	uc := NewMessageUseCase(repo, newMessageTestLogger())

	rows, total, err := uc.ListPage(messageSenderCtx(1, "超管"), 100,
		MessageListCondition{Type: MessageTypeOrder, IsRead: MessageUnread}, 2, 20)
	require.NoError(t, err)
	assert.EqualValues(t, 3, total)
	require.Len(t, rows, 1)

	assert.EqualValues(t, 100, repo.lastListUID, "请求 uid 必须下传到 repo 由其裁决（非超管→403）")
	assert.EqualValues(t, MessageTypeOrder, repo.lastListCond.Type)
	assert.EqualValues(t, MessageUnread, repo.lastListCond.IsRead)
	assert.EqualValues(t, 2, repo.lastListPage)
	assert.EqualValues(t, 20, repo.lastListSize)
}
