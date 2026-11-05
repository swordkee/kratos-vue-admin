/*
 * role_default_router_test.go - 角色「默认菜单(defaultRouter)」回显回归锁。
 *
 * 背景（回灌 third-party/go-kva-admin 3aae32a，源 mall e7d074c）：RoleData 之前没有
 * defaultRouter 字段，ListRoles/FindRoles 都不回填，后台角色页编辑时默认菜单被清空。
 * 本测试锁住「服务层两个读接口都必须回填」，防止字段再次从 pb 或 service 映射里掉出去。
 */
package admin

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	bizadmin "github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	dataadmin "github.com/swordkee/kratos-vue-admin/app/admin/internal/data/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/log"
)

type roleEchoLogger struct{}

func (roleEchoLogger) Log(level log.Level, keyvals ...any) error { return nil }

func newRoleEchoService(t *testing.T) (*RolesService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(10000)", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.SysRoles{}, &model.SysRoleMenus{}, &model.SysMenus{}))

	q := dao.Use(db)
	uc := bizadmin.NewSysRoleUseCase(
		dataadmin.NewSysRoleRepo(q, roleEchoLogger{}),
		roleEchoLogger{},
		nil, nil, nil, nil,
		dataadmin.NewSysMenuRepo(q, roleEchoLogger{}),
	)
	return NewRolesService(uc, roleEchoLogger{}, nil), db
}

func TestRolesServiceEchoesDefaultRouter(t *testing.T) {
	svc, db := newRoleEchoService(t)

	role := &model.SysRoles{
		RoleName:      "内容运营",
		RoleKey:       "content_ops",
		Status:        1,
		RoleSort:      2,
		DataScope:     1,
		DefaultRouter: "/index",
	}
	require.NoError(t, db.Create(role).Error)

	// FindRoles（编辑回显入口）
	find, err := svc.FindRoles(context.Background(), &pb.FindRolesRequest{Id: role.ID})
	require.NoError(t, err)
	require.NotNil(t, find.Role)
	assert.Equal(t, "/index", find.Role.DefaultRouter)
	assert.Equal(t, role.RoleKey, find.Role.RoleKey)

	// ListRoles（列表入口）
	list, err := svc.ListRoles(context.Background(), &pb.ListRolesRequest{PageNum: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, list.Data, 1)
	assert.Equal(t, "/index", list.Data[0].DefaultRouter)
}