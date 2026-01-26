package data

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-kratos/kratos/v2/log"
	pb "github.com/swordkee/kratos-vue-admin/api/admin/v1"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/pkg/util"
)

type DeptIdList struct {
	DeptId int64 `json:"deptId"`
}

type sysDeptRepo struct {
	data *Data
	log  *log.Helper
}

func NewSysDeptRepo(data *Data, logger log.Logger) biz.SysDeptRepo {
	return &sysDeptRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (d *sysDeptRepo) Create(ctx context.Context, dept *model.SysDepts) error {
	q := d.data.Query(ctx).SysDepts
	return q.WithContext(ctx).Create(dept)
}

func (d *sysDeptRepo) Save(ctx context.Context, dept *model.SysDepts) error {
	q := d.data.Query(ctx).SysDepts
	return q.WithContext(ctx).Save(dept)
}

func (d *sysDeptRepo) UpdateByID(ctx context.Context, id int64, dept *model.SysDepts) error {
	if id == 0 {
		return errors.New("user can not update without id")
	}
	q := d.data.Query(ctx).SysDepts
	dataMap := make(map[string]interface{})
	dataMap["parent_id"] = dept.ParentID
	dataMap["dept_name"] = dept.DeptName
	dataMap["leader"] = dept.Leader
	dataMap["phone"] = dept.Phone
	dataMap["email"] = dept.Email
	dataMap["sort"] = dept.Sort
	dataMap["status"] = dept.Status
	_, err := q.WithContext(ctx).Debug().Where(q.ID.Eq(id)).Updates(dataMap)
	return err
}

func (d *sysDeptRepo) Delete(ctx context.Context, id int64) error {
	q := d.data.Query(ctx).SysDepts
	_, err := q.WithContext(ctx).Where(q.ID.Eq(id)).Delete()
	return err

}

func (d *sysDeptRepo) FindByID(ctx context.Context, id int64) (*model.SysDepts, error) {
	q := d.data.Query(ctx).SysDepts
	return q.WithContext(ctx).Where(q.ID.Eq(id)).First()
}

func (d *sysDeptRepo) ListByNameStatusId(ctx context.Context, deptName string, status int32, id int64) ([]*model.SysDepts, error) {
	q := d.data.Query(ctx).SysDepts
	db := q.WithContext(ctx)
	if deptName != "" {
		db = db.Where(q.DeptName.Like(fmt.Sprintf("%%%s%%", deptName)))
	}
	if status != 0 {
		db = db.Where(q.Status.Eq(status))
	}
	if id != 0 {
		db = db.Where(q.ID.Eq(id))
	}
	return db.Find()
}

func (d *sysDeptRepo) GetRoleDeptId(ctx context.Context, roleId int32) ([]int32, error) {
	deptIds := make([]int32, 0)

	// 使用 gorm gen 替代 Table
	roleDeptsQ := d.data.Query(ctx).SysRoleDepts
	sysDeptQ := d.data.Query(ctx).SysDepts

	// 获取角色关联的部门
	roleDepts, err := roleDeptsQ.WithContext(ctx).
		Select(roleDeptsQ.DeptID).
		Where(roleDeptsQ.RoleID.Eq(int64(roleId))).
		Find()
	if err != nil {
		return nil, err
	}

	// 构建子查询：不包含作为父部门的部门ID
	var excludeIds []int64
	for _, rd := range roleDepts {
		children, _ := sysDeptQ.WithContext(ctx).
			Where(sysDeptQ.ParentID.Eq(int64(rd.DeptID))).
			Select(sysDeptQ.ID).
			Find()
		for _, child := range children {
			excludeIds = append(excludeIds, int64(child.ID))
		}
	}

	// 获取最终部门ID列表
	for _, rd := range roleDepts {
		isExcluded := false
		for _, exId := range excludeIds {
			if int64(rd.DeptID) == exId {
				isExcluded = true
				break
			}
		}
		if !isExcluded {
			deptIds = append(deptIds, int32(rd.DeptID))
		}
	}

	return deptIds, nil
}

func (d *sysDeptRepo) FindByIDList(ctx context.Context, ids ...int64) ([]*model.SysDepts, error) {
	q := d.data.Query(ctx).SysDepts
	return q.WithContext(ctx).Where(q.ID.In(ids...)).Find()
}

func (d *sysDeptRepo) SelectDept(ctx context.Context) ([]*pb.DeptTree, error) {
	q := d.data.Query(ctx).SysDepts
	deptList, err := q.WithContext(ctx).Find()

	if err != nil {
		return nil, err
	}

	dl := make([]*pb.DeptTree, 0)
	for i := 0; i < len(deptList); i++ {
		if deptList[i].ParentID != 0 {
			continue
		}
		e := &pb.DeptTree{}
		e.DeptId = deptList[i].ID
		e.DeptName = deptList[i].DeptName
		e.DeptPath = deptList[i].DeptPath
		e.ParentId = deptList[i].ParentID
		e.Sort = deptList[i].Sort
		e.Leader = deptList[i].Leader
		e.Phone = deptList[i].Phone
		e.Email = deptList[i].Email
		e.Status = deptList[i].Status
		e.CreateBy = deptList[i].CreateBy
		e.UpdateBy = deptList[i].UpdateBy
		e.CreateTime = util.NewTimestamp(deptList[i].CreatedAt)
		e.UpdateTime = util.NewTimestamp(deptList[i].UpdatedAt)

		deptListInfo := recursiveDept(deptList, e)

		dl = append(dl, deptListInfo)
	}
	return dl, nil
}

// RecursiveDept 递归 dept
func recursiveDept(deptList []*model.SysDepts, dept *pb.DeptTree) *pb.DeptTree {
	list := deptList
	min := make([]*pb.DeptTree, 0)
	for j := 0; j < len(list); j++ {
		if dept.DeptId != list[j].ParentID {
			continue
		}
		mi := &pb.DeptTree{
			DeptId:     list[j].ID,
			ParentId:   list[j].ParentID,
			DeptPath:   list[j].DeptPath,
			DeptName:   list[j].DeptName,
			Leader:     list[j].Leader,
			Phone:      list[j].Phone,
			Email:      list[j].Email,
			Status:     list[j].Status,
			CreateTime: util.NewTimestamp(list[j].CreatedAt),
			UpdateTime: util.NewTimestamp(list[j].UpdatedAt),
			Children:   []*pb.DeptTree{},
		}
		ms := recursiveDept(deptList, mi)
		min = append(min, ms)
	}
	dept.Children = min
	return dept
}

func (d *sysDeptRepo) SelectDeptLabel(ctx context.Context) ([]*pb.DeptLabel, error) {
	deptList, err := d.ListByNameStatusId(ctx, "", 0, 0)
	if err != nil {
		return nil, err
	}

	dl := make([]*pb.DeptLabel, 0)

	for i := 0; i < len(deptList); i++ {
		if deptList[i].ParentID != 0 {
			continue
		}
		e := &pb.DeptLabel{}
		e.DeptId = int32(deptList[i].ID)
		e.DeptName = deptList[i].DeptName
		deptsInfo := RecursiveDeptLabel(deptList, e)

		dl = append(dl, deptsInfo)
	}
	return dl, nil
}

func RecursiveDeptLabel(deptList []*model.SysDepts, dept *pb.DeptLabel) *pb.DeptLabel {
	list := deptList
	min := make([]*pb.DeptLabel, 0)
	for j := 0; j < len(list); j++ {
		if dept.DeptId != int32(list[j].ParentID) {
			continue
		}
		mi := &pb.DeptLabel{
			DeptId:   int32(list[j].ID),
			DeptName: list[j].DeptName,
			Children: []*pb.DeptLabel{},
		}
		ms := RecursiveDeptLabel(deptList, mi)
		min = append(min, ms)
	}
	dept.Children = min
	return dept
}
