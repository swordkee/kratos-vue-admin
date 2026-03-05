package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// SysRoleMenuRepo 接口定义
type SysRoleMenuRepo interface {
	Create(ctx context.Context, roleMenus ...*model.SysRoleMenus) error
	DeleteByRoleId(ctx context.Context, roleIDs ...int64) error
	GetPermission(ctx context.Context, roleID int64) ([]string, error)
	FindMenuByRoleId(ctx context.Context, roleID int64) ([]*model.SysMenus, error)
	SelectMenuRole(ctx context.Context, roleName string) ([]*pb.MenuTree, error)
}

type SysRoleMenuUseCase struct {
	repo SysRoleMenuRepo
	log  *log.Helper
}

func NewSysRoleMenuUseCase(repo SysRoleMenuRepo, logger log.Logger) *SysRoleMenuUseCase {
	return &SysRoleMenuUseCase{repo: repo, log: log.NewHelper(logger)}
}

func (r *SysRoleMenuUseCase) CreateRoleMenus(ctx context.Context, role *model.SysRoles, menuIDs []int64) error {
	roleMenus := make([]*model.SysRoleMenus, len(menuIDs))
	for i, menuID := range menuIDs {
		roleMenus[i] = &model.SysRoleMenus{
			MenuID:   menuID,
			RoleID:   role.ID,
			RoleName: role.RoleName,
		}
	}
	return r.repo.Create(ctx, roleMenus...)
}

func (r *SysRoleMenuUseCase) DeleteByRoleId(ctx context.Context, roleIDs ...int64) error {
	return r.repo.DeleteByRoleId(ctx, roleIDs...)
}

func (r *SysRoleMenuUseCase) FindPermission(ctx context.Context, roleID int64) ([]string, error) {
	return r.repo.GetPermission(ctx, roleID)
}

func (r *SysRoleMenuUseCase) FindMenuByRoleId(ctx context.Context, roleID int64) ([]*model.SysMenus, error) {
	return r.repo.FindMenuByRoleId(ctx, roleID)
}

func (r *SysRoleMenuUseCase) SelectMenuRole(ctx context.Context, roleName string) ([]*pb.MenuTree, error) {
	return r.repo.SelectMenuRole(ctx, roleName)
}
