package data

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

type MenuIdList struct {
	ID int64 `json:"id"`
}

type sysMenuRepo struct {
	data *Data
	log  *log.Helper
}

func NewSysMenuRepo(data *Data, logger log.Logger) biz.SysMenuRepo {
	return &sysMenuRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (m *sysMenuRepo) Create(ctx context.Context, menu *model.SysMenus) error {
	q := m.data.Query(ctx).SysMenus
	return q.WithContext(ctx).Create(menu)
}

func (m *sysMenuRepo) Save(ctx context.Context, menu *model.SysMenus) error {
	q := m.data.Query(ctx).SysMenus
	return q.WithContext(ctx).Save(menu)
}

func (m *sysMenuRepo) Delete(ctx context.Context, id int64) error {
	q := m.data.Query(ctx).SysMenus
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err
}

func (m *sysMenuRepo) GetAllChildren(ctx context.Context, id int64) ([]int64, error) {
	q := m.data.Query(ctx).SysMenus
	// 要返回的子集菜单
	var allChildrenMenusIds []int64
	nextParentIds := []int64{id}
	for len(nextParentIds) > 0 {
		menus, err := q.WithContext(ctx).Where(q.ParentID.In(nextParentIds...)).Find()
		if err != nil {
			return nil, err
		}
		nextParentIds = nil
		for _, menu := range menus {
			allChildrenMenusIds = append(allChildrenMenusIds, menu.ID)
			nextParentIds = append(nextParentIds, menu.ID)
		}
	}
	return allChildrenMenusIds, nil
}

func (m *sysMenuRepo) DeleteMultiple(ctx context.Context, ids []int64) error {
	q := m.data.Query(ctx).SysMenus
	_, err := q.WithContext(ctx).Where(q.ID.In(ids...)).Delete()
	return err
}

func (m *sysMenuRepo) FindById(ctx context.Context, id int64) (*model.SysMenus, error) {
	q := m.data.Query(ctx).SysMenus
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (m *sysMenuRepo) ListAll(ctx context.Context) ([]*model.SysMenus, error) {
	q := m.data.Query(ctx).SysMenus
	return q.WithContext(ctx).Find()
}

func (m *sysMenuRepo) FindByNameStatus(ctx context.Context, name string, status int32) ([]*model.SysMenus, error) {
	q := m.data.Query(ctx).SysMenus
	db := q.WithContext(ctx)
	if name != "" {
		db = db.Where(q.MenuName.Like(fmt.Sprintf("%%%s%%", name)))
	}
	if status != 0 {
		db = db.Where(q.Status.Eq(status))
	}
	return db.Order(q.Sort).Find()
}

// GetRoleMenuId 获取角色对应的菜单ids
func (m *sysMenuRepo) GetRoleMenuId(ctx context.Context, roleId int64) ([]int32, error) {
	menuIds := make([]int32, 0)

	query := m.data.Query(ctx)
	roleMenu := query.SysRoleMenus
	menu := query.SysMenus

	// 获取角色关联的菜单
	menus, err := menu.WithContext(ctx).
		Select(menu.ID).
		LeftJoin(roleMenu, menu.ID.EqCol(roleMenu.MenuID)).
		Where(roleMenu.RoleID.Eq(roleId)).
		Find()
	if err != nil {
		return nil, err
	}

	for _, menu := range menus {
		menuIds = append(menuIds, int32(menu.ID))
	}
	return menuIds, nil
}

func (m *sysMenuRepo) SelectMenuLabel(ctx context.Context, data model.SysMenus) ([]*pb.MenuLabel, error) {
	menuList, err := m.FindList(ctx, data)
	if err != nil {
		return nil, err
	}

	redData := make([]*pb.MenuLabel, 0)
	ml := menuList
	for i := 0; i < len(ml); i++ {
		if ml[i].ParentID != 0 {
			continue
		}
		e := &pb.MenuLabel{}
		e.MenuId = int32(ml[i].ID)
		e.MenuName = ml[i].MenuName
		menusInfo := DiguiMenuLabel(menuList, e)

		redData = append(redData, menusInfo)
	}
	return redData, err
}

func (m *sysMenuRepo) FindList(ctx context.Context, data model.SysMenus) ([]*model.SysMenus, error) {
	list := make([]*model.SysMenus, 0)

	q := m.data.Query(ctx).SysMenus
	db := q.WithContext(ctx)
	// 此处填写 where参数判断
	if data.MenuName != "" {
		db = db.Where(q.MenuName.Like(fmt.Sprintf("%%%s%%", data.MenuName)))
	}
	if data.Path != "" {
		db = db.Where(q.Path.Eq(data.Path))
	}
	if data.MenuType != "" {
		db = db.Where(q.MenuType.Eq(data.MenuType))
	}
	if data.Title != "" {
		db = db.Where(q.Title.Like(fmt.Sprintf("%%%s%%", data.Title)))
	}
	if data.Status != 0 {
		db = db.Where(q.Status.Eq(data.Status))
	}
	db = db.Where(q.DeletedAt.IsNull())
	list, err := db.Order(q.Sort).Find()
	if err != nil {
		return nil, err
	}
	return list, nil
}

func DiguiMenu(menulist []*model.SysMenus, menu *pb.MenuTree) *pb.MenuTree {
	list := menulist

	min := make([]*pb.MenuTree, 0)
	for j := 0; j < len(list); j++ {

		if menu.MenuId != list[j].ParentID {
			continue
		}
		mi := &pb.MenuTree{}
		mi.MenuId = list[j].ID
		mi.MenuName = list[j].MenuName
		mi.Title = list[j].Title
		mi.Icon = list[j].Icon
		mi.Path = list[j].Path
		mi.MenuType = list[j].MenuType
		mi.IsKeepAlive = list[j].KeepAlive
		mi.Permission = list[j].Permission
		mi.ParentId = list[j].ParentID
		mi.IsAffix = list[j].IsAffix
		mi.IsIframe = list[j].IsIframe
		mi.IsLink = list[j].Link
		mi.Component = list[j].Component
		mi.Sort = list[j].Sort
		mi.Status = list[j].Status
		mi.IsHide = list[j].Hidden
		mi.CreateTime = util.NewTimestamp(list[j].CreatedAt)
		mi.UpdateTime = util.NewTimestamp(list[j].UpdatedAt)
		mi.Children = []*pb.MenuTree{}

		if mi.MenuType != "F" {
			ms := DiguiMenu(menulist, mi)
			min = append(min, ms)
		} else {
			min = append(min, mi)
		}
	}
	menu.Children = min
	return menu
}

func DiguiMenuLabel(menulist []*model.SysMenus, menu *pb.MenuLabel) *pb.MenuLabel {
	list := menulist

	min := make([]*pb.MenuLabel, 0)
	for j := 0; j < len(list); j++ {

		if menu.MenuId != int32(list[j].ParentID) {
			continue
		}
		mi := pb.MenuLabel{}
		mi.MenuId = int32(list[j].ID)
		mi.MenuName = list[j].MenuName
		mi.Children = []*pb.MenuLabel{}
		if list[j].MenuType != "F" {
			ms := DiguiMenuLabel(menulist, &mi)
			min = append(min, ms)
		} else {
			min = append(min, &mi)
		}

	}
	menu.Children = min
	return menu
}
