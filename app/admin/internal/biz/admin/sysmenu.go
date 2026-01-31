package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

// SysMenuRepo 接口定义
type SysMenuRepo interface {
	Save(ctx context.Context, menu *model.SysMenus) error
	Create(ctx context.Context, menu *model.SysMenus) error
	Delete(ctx context.Context, id int64) error
	DeleteMultiple(ctx context.Context, ids []int64) error
	FindByID(ctx context.Context, id int64) (*model.SysMenus, error)
	Count(ctx context.Context, name string, status int32) (int32, error)
	GetAllChildren(ctx context.Context, id int64) ([]int64, error)
	FindByNameStatus(ctx context.Context, menuName string, status int32) ([]*model.SysMenus, error)
	SelectMenuLabel(ctx context.Context, menu model.SysMenus) ([]*pb.MenuLabel, error)
	GetRoleMenuId(ctx context.Context, roleId int64) ([]int32, error)
}

type SysMenuUseCase struct {
	repo SysMenuRepo
	log  *log.Helper
}

func NewSysMenusUseCase(repo SysMenuRepo, logger log.Logger) *SysMenuUseCase {
	return &SysMenuUseCase{repo: repo, log: log.NewHelper(logger)}
}

func (m *SysMenuUseCase) CreateMenus(ctx context.Context, menu *model.SysMenus) (*model.SysMenus, error) {
	claims := authz.MustFromContext(ctx)
	menu.CreateBy = claims.Nickname

	err := m.repo.Create(ctx, menu)
	return menu, err
}

func (m *SysMenuUseCase) UpdateMenus(ctx context.Context, menu *model.SysMenus) (*model.SysMenus, error) {
	claims := authz.MustFromContext(ctx)
	menu.UpdateBy = claims.Nickname

	err := m.repo.Save(ctx, menu)
	return menu, err
}

func (m *SysMenuUseCase) DeleteMenus(ctx context.Context, id int64) error {
	// 删除父级菜单时同时删除子菜单，否则获取菜单会报错
	allChildrenMenus, err := m.repo.GetAllChildren(ctx, id)
	if err != nil {
		return pb.ErrorDatabaseErr("获取所有子菜单失败:%s", err.Error())
	}
	err = m.repo.DeleteMultiple(ctx, allChildrenMenus)
	if err != nil {
		return pb.ErrorDatabaseErr("删除子菜单失败:%s", err.Error())
	}
	return m.repo.Delete(ctx, id)
}

func (m *SysMenuUseCase) FindMenus(ctx context.Context, id int64) (*model.SysMenus, error) {
	return m.repo.FindByID(ctx, id)
}

type MenuSimpleTree struct {
	MenuId   int64             `json:"menuId"`
	MenuName string            `json:"menuName"`
	Children []*MenuSimpleTree `json:"children,omitempty"`
}

type MenuTree struct {
	model.SysMenus
	Children []*MenuTree `json:"children,omitempty"`
}

func (m *SysMenuUseCase) ListByNameStatus(ctx context.Context, menuName string, status int32) ([]*model.SysMenus, error) {
	return m.repo.FindByNameStatus(ctx, menuName, status)
}

func (m *SysMenuUseCase) RoleMenuTreeSelect(ctx context.Context, req *pb.RoleMenuTreeSelectRequest) (*pb.RoleMenuTreeSelectReply, error) {
	var err error
	result, err := m.repo.SelectMenuLabel(ctx, model.SysMenus{})
	if err != nil {
		return nil, err
	}
	menuIds := make([]int32, 0)
	if req.RoleId != 0 {
		menuIds, err = m.repo.GetRoleMenuId(ctx, req.RoleId)
		if err != nil {
			return nil, err
		}
	}
	reply := &pb.RoleMenuTreeSelectReply{
		Menus:       result,
		CheckedKeys: menuIds,
	}
	return reply, err
}
