package media

import (
	"context"

	"github.com/swordkee/kratos-vue-admin/pkg/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// CategoryUseCase 媒体库分类用例（源 AttachmentCategoryService）。
type CategoryUseCase struct {
	repo CategoryRepo
	l    *log.Helper
}

// NewCategoryUseCase 构造分类用例。
func NewCategoryUseCase(repo CategoryRepo, logger log.Logger) *CategoryUseCase {
	return &CategoryUseCase{repo: repo, l: newHelper(logger)}
}

// List 全量分类树（源 getChildrenList：根 pid=0）。
func (uc *CategoryUseCase) List(ctx context.Context) ([]*CategoryNode, error) {
	rows, err := uc.repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	return buildCategoryTree(rows, 0), nil
}

// Add 创建/更新分类（id>0 更新）。
//
// 源实现先做 Take(name=? and pid=?) 判重且不排除自身 ⇒ 编辑分类时只要不改名就会被判定重名。
// 本仓按「同根因同方案」修正为排除自身（e2e 锁：保持原名更新必须成功）。
func (uc *CategoryUseCase) Add(ctx context.Context, id int64, name string, pid int64) (*model.ExaAttachmentCategory, error) {
	if _, err := uc.repo.FindByNameAndPid(ctx, name, pid, id); err == nil {
		return nil, ErrCategoryNameDuplicated
	}
	if id > 0 {
		if err := uc.repo.UpdateNamePid(ctx, id, name, pid); err != nil {
			return nil, err
		}
		return &model.ExaAttachmentCategory{ID: id, Name: name, Pid: int32(pid)}, nil
	}
	row := &model.ExaAttachmentCategory{Name: name, Pid: int32(pid)}
	if err := uc.repo.Create(ctx, row); err != nil {
		return nil, err
	}
	return row, nil
}

// Delete 删除分类（有子级拒绝；Unscoped 物理删）。
func (uc *CategoryUseCase) Delete(ctx context.Context, id int64) error {
	n, err := uc.repo.CountChildren(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrCategoryHasChildren
	}
	return uc.repo.DeleteHard(ctx, id)
}

func buildCategoryTree(rows []*model.ExaAttachmentCategory, parentID int32) []*CategoryNode {
	var tree []*CategoryNode
	for _, r := range rows {
		if r.Pid == parentID {
			tree = append(tree, &CategoryNode{
				ExaAttachmentCategory: r,
				Children:              buildCategoryTree(rows, int32(r.ID)),
			})
		}
	}
	return tree
}
