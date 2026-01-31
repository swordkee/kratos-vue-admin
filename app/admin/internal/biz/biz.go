package biz

import (
	"context"
	"time"

	"github.com/google/wire"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/model"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	admin.NewSysUserUseCase,
	admin.NewAuthUseCase,
	admin.NewSysMenusUseCase,
	admin.NewSysDeptUseCase,
	admin.NewSysPostUseCase,
	admin.NewSysApiUseCase,
	admin.NewSysRoleUseCase,
	admin.NewSysRoleMenuUseCase,
	admin.NewCasbinRuleUseCase,
	admin.NewSysDictDatumUseCase,
	admin.NewSysDictTypeUseCase,
	admin.NewSysLogsUseCase,
)

// Transaction 事务接口类型别名（指向 admin.Transaction 以避免循环导入）
type Transaction = admin.Transaction

type RedisRepo interface {
	SetHashKey(context.Context, string, string, interface{}) error
	GetHashKey(context.Context, string, string) (string, error)
	DelHashKey(ctx context.Context, key string, field string) error
	GetHashLen(ctx context.Context, key string) error
	Lock(context.Context, string, interface{}, time.Duration) (bool, error)
	IncrHashKey(context.Context, string, string, int64) error
	GetHashAllKeyAndVal(context.Context, string) (map[string]string, error)
	Set(ctx context.Context, key string, value string, expire time.Duration) error
	Get(ctx context.Context, key string) string
	SRem(ctx context.Context, key string, members ...interface{}) (int64, error)
}

type OssRepo interface {
	UploadFile(file interface{}, filePath string) (string, error)
}

// UserListCondition is a condition for user list query.
type UserListCondition struct {
	UserName string
	Phone    string
	Status   int32
}

// Repo interfaces
type SysUserRepo interface {
	Save(ctx context.Context, user *model.SysUsers) (*model.SysUsers, error)
	Delete(ctx context.Context, id int64) error
	UpdateByID(ctx context.Context, id int64, user *model.SysUsers) error
	Create(ctx context.Context, g *model.SysUsers) (*model.SysUsers, error)
	FindByID(ctx context.Context, id int64) (*model.SysUsers, error)
	FindByUsername(ctx context.Context, username string) (*model.SysUsers, error)
	FindByPostId(ctx context.Context, postId int64) ([]*model.SysUsers, error)
	ListPage(ctx context.Context, page, size int32, condition UserListCondition) ([]*model.SysUsers, error)
	Count(ctx context.Context, condition UserListCondition) (int32, error)
	CountByRoleId(ctx context.Context, roleId int64) (int64, error)
	FindAll(ctx context.Context) ([]*model.SysUsers, error)
}

type SysRoleRepo interface {
	Create(ctx context.Context, role *model.SysRoles) error
	Save(ctx context.Context, role *model.SysRoles) error
	Delete(ctx context.Context, id ...int64) error
	FindByID(ctx context.Context, id int64) (*model.SysRoles, error)
	FindByIDList(ctx context.Context, ids ...int64) ([]*model.SysRoles, error)
	FindAll(ctx context.Context) ([]*model.SysRoles, error)
	ListPage(ctx context.Context, name, key string, status int32, page, size int32) ([]*model.SysRoles, error)
	Count(ctx context.Context, name, key string, status int32) (int32, error)
	Update(ctx context.Context, role *model.SysRoles) error
}

type SysMenuRepo interface {
	Save(ctx context.Context, menu *model.SysMenus) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysMenus, error)
	FindAll(ctx context.Context) ([]*model.SysMenus, error)
	ListPage(ctx context.Context, name string, status int32, page, size int32) ([]*model.SysMenus, error)
	Count(ctx context.Context, name string, status int32) (int32, error)
	FindByRoleID(ctx context.Context, roleId int64) ([]*model.SysMenus, error)
}

type SysDeptRepo interface {
	Save(ctx context.Context, dept *model.SysDepts) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysDepts, error)
	FindAll(ctx context.Context) ([]*model.SysDepts, error)
	FindByParentID(ctx context.Context, parentID int64) ([]*model.SysDepts, error)
}

type SysPostRepo interface {
	Save(ctx context.Context, post *model.SysDepts) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysDepts, error)
	FindAll(ctx context.Context) ([]*model.SysDepts, error)
}

type SysApiRepo interface {
	FindByID(ctx context.Context, id int64) (*model.SysApis, error)
	Create(ctx context.Context, api *model.SysApis) error
	Save(ctx context.Context, api *model.SysApis) error
	Delete(ctx context.Context, id int64) error
	FindAll(ctx context.Context) ([]*model.SysApis, error)
	ListPage(ctx context.Context, page, size int32) ([]*model.SysApis, error)
	ListPageCount(ctx context.Context) (int32, error)
}

type SysDictDataRepo interface {
	Save(ctx context.Context, dict *model.SysDictData) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysDictData, error)
	FindAll(ctx context.Context) ([]*model.SysDictData, error)
	ListPage(ctx context.Context, label string, status int32, page, size int32) ([]*model.SysDictData, error)
	Count(ctx context.Context, label string, status int32) (int32, error)
	FindByType(ctx context.Context, dictType string) ([]*model.SysDictData, error)
}

type SysDictTypeRepo interface {
	Save(ctx context.Context, dictType *model.SysDictTypes) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysDictTypes, error)
	FindAll(ctx context.Context) ([]*model.SysDictTypes, error)
	ListPage(ctx context.Context, label string, status int32, page, size int32) ([]*model.SysDictTypes, error)
	Count(ctx context.Context, label string, status int32) (int32, error)
}

type SysLogRepo interface {
	Save(ctx context.Context, log *model.SysLogs) error
	Delete(ctx context.Context, id int64) error
	FindByID(ctx context.Context, id int64) (*model.SysLogs, error)
	FindAll(ctx context.Context) ([]*model.SysLogs, error)
	ListPage(ctx context.Context, page, size int32) ([]*model.SysLogs, error)
	Count(ctx context.Context) (int32, error)
}

// UseCase 类型别名
type SysUserUseCase = admin.SysUserUseCase
type SysRoleUseCase = admin.SysRoleUseCase
type SysMenuUseCase = admin.SysMenuUseCase
type SysDeptUseCase = admin.SysDeptUseCase
type SysPostUseCase = admin.SysPostUseCase
type SysApiUseCase = admin.SysApiUseCase
type SysDictDatumUseCase = admin.SysDictDatumUseCase
type SysDictTypeUseCase = admin.SysDictTypeUseCase
type SysRoleMenuUseCase = admin.SysRoleMenuUseCase
type SysLogsUseCase = admin.SysLogsUseCase

// 函数别名
var ConvertToDeptTree = admin.ConvertToDeptTree
var ConvertToDeptTreeChildren = admin.ConvertToDeptTreeChildren
