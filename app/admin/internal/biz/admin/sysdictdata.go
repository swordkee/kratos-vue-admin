package admin

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"

	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/pkg/authz"
)

// SysDictDataRepo 接口定义
type SysDictDataRepo interface {
	Save(ctx context.Context, dict *model.SysDictData) error
	Create(ctx context.Context, dict *model.SysDictData) error
	Delete(ctx context.Context, id []int64) error
	FindByID(ctx context.Context, id int64) (*model.SysDictData, error)
	FindByIDList(ctx context.Context, ids ...int64) ([]*model.SysDictData, error)
	FindAll(ctx context.Context) ([]*model.SysDictData, error)
	ListPage(ctx context.Context, dictLabel, dictType string, status int32, page, size int32) ([]*model.SysDictData, error)
	Count(ctx context.Context, label string, status int32) (int32, error)
	ListPageCount(ctx context.Context, dictLabel, dictType string, status int32) (int32, error)
}

type SysDictDatumUseCase struct {
	repo SysDictDataRepo
	log  *log.Helper
}

func NewSysDictDatumUseCase(repo SysDictDataRepo, logger log.Logger) *SysDictDatumUseCase {
	return &SysDictDatumUseCase{repo: repo, log: log.NewHelper(logger)}
}

func (p *SysDictDatumUseCase) ListDictData(ctx context.Context, dictLabel, dictType string, status int32, page, size int32) ([]*model.SysDictData, int32, error) {
	total, err := p.repo.ListPageCount(ctx, dictLabel, dictType, status)
	if err != nil {
		return nil, 0, err
	}
	posts, err := p.repo.ListPage(ctx, dictLabel, dictType, status, page, size)
	return posts, total, err
}

func (p *SysDictDatumUseCase) CreateDictData(ctx context.Context, post *model.SysDictData) (*model.SysDictData, error) {
	claims := authz.MustFromContext(ctx)
	post.CreateBy = claims.Nickname

	err := p.repo.Create(ctx, post)
	return post, err
}

func (p *SysDictDatumUseCase) UpdateDictData(ctx context.Context, post *model.SysDictData) (*model.SysDictData, error) {
	claims := authz.MustFromContext(ctx)
	post.UpdateBy = claims.Nickname
	err := p.repo.Save(ctx, post)
	return post, err
}

func (p *SysDictDatumUseCase) DeleteDictData(ctx context.Context, id []int64) error {
	return p.repo.Delete(ctx, id)
}

func (p *SysDictDatumUseCase) FindDictDataByIDList(ctx context.Context, ids []int64) ([]*model.SysDictData, error) {
	if len(ids) == 0 {
		return []*model.SysDictData{}, nil
	}
	return p.repo.FindByIDList(ctx, ids...)
}

func (p *SysDictDatumUseCase) FindDictDataByID(ctx context.Context, id int64) (*model.SysDictData, error) {
	return p.repo.FindByID(ctx, id)
}

func (p *SysDictDatumUseCase) FindDictDataAll(ctx context.Context) ([]*model.SysDictData, error) {
	return p.repo.FindAll(ctx)
}
