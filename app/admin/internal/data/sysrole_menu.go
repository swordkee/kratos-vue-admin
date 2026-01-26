package data

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

/**
  菜单类型（M目录 C菜单 F按钮）
  菜单类型 (1-目录 2-菜单 3-按钮）
*/

type sysRoleMenuRepo struct {
	data *Data
	log  *log.Helper
}

func NewSysRoleMenuRepo(data *Data, logger log.Logger) biz.SysRoleMenuRepo {
	return &sysRoleMenuRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (s *sysRoleMenuRepo) Create(ctx context.Context, roleMenus ...*model.SysRoleMenus) error {
	q := s.data.Query(ctx).SysRoleMenus
	return q.WithContext(ctx).Create(roleMenus...)
}

func (s *sysRoleMenuRepo) DeleteByRoleId(ctx context.Context, roleIDs ...int64) error {
	q := s.data.Query(ctx).SysRoleMenus
	_, err := q.WithContext(ctx).Where(q.RoleID.In(roleIDs...)).Delete()
	return err
}

// GetPermission 查询权限标识
func (s *sysRoleMenuRepo) GetPermission(ctx context.Context, roleID int64) ([]string, error) {
	query := s.data.Query(ctx)
	roleMenu := query.SysRoleMenus
	menu := query.SysMenus

	var result []string
	err := menu.WithContext(ctx).
		Select(menu.Permission).
		LeftJoin(roleMenu, menu.ID.EqCol(roleMenu.MenuID)).
		Where(roleMenu.RoleID.Eq(roleID)).
		Where(menu.MenuType.In("C", "F")).Scan(&result)
	return result, err
}

// FindMenuByRoleId 查询菜单路径
func (s *sysRoleMenuRepo) FindMenuByRoleId(ctx context.Context, roleID int64) ([]*model.SysMenus, error) {
	query := s.data.Query(ctx)
	roleMenu := query.SysRoleMenus
	menu := query.SysMenus

	return menu.WithContext(ctx).
		LeftJoin(roleMenu, menu.ID.EqCol(roleMenu.MenuID)).
		Where(roleMenu.RoleID.Eq(roleID)).
		Where(menu.MenuType.In("M", "C")).
		Order(menu.Sort).
		Find()
}

func (s *sysRoleMenuRepo) SelectMenuRole(ctx context.Context, roleKey string) ([]*pb.MenuTree, error) {
	redData := make([]*pb.MenuTree, 0)

	menuList, err := s.GetMenuByRoleKey(ctx, roleKey)
	if err != nil {
		return nil, err
	}

	for i := 0; i < len(menuList); i++ {
		if menuList[i].ParentID != 0 {
			continue
		}
		menuTree := &pb.MenuTree{}
		_ = util.CopyStructFields(menuTree, menuList[i])
		menuTree.MenuId = menuList[i].ID

		menusInfo := DiguiMenu(menuList, menuTree)

		redData = append(redData, menusInfo)
	}
	return redData, nil
}

func (s *sysRoleMenuRepo) GetMenuByRoleKey(ctx context.Context, roleKey string) ([]*model.SysMenus, error) {
	menus := make([]*model.SysMenus, 0)

	query := s.data.Query(ctx)
	roleMenu := query.SysRoleMenus
	menu := query.SysMenus

	menus, err := menu.WithContext(ctx).
		Select(menu.ALL).
		LeftJoin(roleMenu, menu.ID.EqCol(roleMenu.MenuID)).
		Where(roleMenu.RoleName.Eq(roleKey)).
		Where(menu.MenuType.In("M", "C")).
		Where(menu.Status.In(1, 0)).
		Order(menu.Sort).
		Find()
	if err != nil {
		return nil, err
	}
	return menus, nil
}
