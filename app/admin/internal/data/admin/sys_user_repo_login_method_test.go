/*
 * sys_user_repo_login_method_test.go - sys_users.login_method 列读写锁（内存 SQLite）
 *
 * 背景：2026-11-05 新增「用户级登录方式」（1=TOTP 默认 / 2=手机验证码短信）。
 * gen 模型与 DAO 已按实库列定义对齐（LoginMethod int32），本测试锁两件事：
 *  1. Create 落库后读回一致（DAO 字段映射含 login_method）；
 *  2. UpdateByID 能改写入 login_method（后台改登录方式必须真正生效）。
 */
package admin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	adminbiz "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

func newSysUserRepoFixture(t *testing.T) (*SysUserRepo, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(10000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysUsers{}))
	return NewSysUserRepo(dao.Use(db), db, log.DefaultLogger).(*SysUserRepo), db
}

func TestSysUserRepo_LoginMethodReadWrite(t *testing.T) {
	repo, _ := newSysUserRepoFixture(t)
	ctx := context.Background()

	now := time.Now()
	created, err := repo.Create(ctx, &model.SysUsers{
		UUID:        "uuid-login-method",
		Username:    "lm_repo",
		NickName:    "登录方式用例",
		Password:    "x",
		Phone:       "13800000000",
		RoleID:      1,
		Status:      1,
		LoginMethod: adminbiz.LoginMethodTOTP,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	got, err := repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.EqualValues(t, adminbiz.LoginMethodTOTP, got.LoginMethod, "Create 应落库 login_method=1")

	// 登录方式切换到短信
	require.NoError(t, repo.UpdateByID(ctx, created.ID, &model.SysUsers{
		LoginMethod: adminbiz.LoginMethodSMS,
		UpdateBy:    "tester",
		UpdatedAt:   time.Now(),
	}))

	got, err = repo.FindByID(ctx, created.ID)
	require.NoError(t, err)
	assert.EqualValues(t, adminbiz.LoginMethodSMS, got.LoginMethod, "UpdateByID 必须能改写 login_method")
}

// 列表/详情读路径不得因新列炸掉（DAO 字段映射与 model 对齐）。
func TestSysUserRepo_ListPageCarriesLoginMethod(t *testing.T) {
	repo, _ := newSysUserRepoFixture(t)
	ctx := context.Background()

	now := time.Now()
	_, err := repo.Create(ctx, &model.SysUsers{
		UUID: "uuid-lm-list", Username: "lm_list", NickName: "列表用例", Password: "x",
		Phone: "13900000001", RoleID: 1, Status: 1, LoginMethod: adminbiz.LoginMethodSMS,
		CreatedAt: now, UpdatedAt: now,
	})
	require.NoError(t, err)

	rows, err := repo.ListPage(ctx, 1, 10, adminbiz.UserListCondition{})
	require.NoError(t, err)
	require.NotEmpty(t, rows)
	assert.EqualValues(t, adminbiz.LoginMethodSMS, rows[0].LoginMethod)
}
