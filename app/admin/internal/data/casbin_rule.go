package data

import (
	"context"
	"errors"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/go-kratos/kratos/v2/log"
	"gorm.io/gorm"

	"github.com/byteflowteam/kratos-vue-admin/app/admin/internal/biz"
)

type casbinRuleRepo struct {
	data           *Data
	log            *log.Helper
	syncedEnforcer *casbin.SyncedCachedEnforcer
}

// 内置的 Casbin 模型配置（使用 server 目录方案）
const builtinCasbinModel = `
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = r.sub == p.sub && keyMatch2(r.obj, p.obj) && r.act == p.act
`

var (
	syncedCachedEnforcer *casbin.SyncedCachedEnforcer
	once                 sync.Once
)

func NewCasbinRuleRepo(data *Data, db *gorm.DB, logger log.Logger) biz.CasbinRuleRepo {
	once.Do(func() {
		adapter, err := gormadapter.NewAdapterByDB(db)
		if err != nil {
			panic("新建权限适配器失败:\n" + err.Error())
		}

		text := builtinCasbinModel
		m, err := model.NewModelFromString(text)
		if err != nil {
			panic("加载内置模型失败:\n" + err.Error())
		}

		syncedCachedEnforcer, err = casbin.NewSyncedCachedEnforcer(m, adapter)
		if err != nil {
			panic("新建权限引擎失败:\n" + err.Error())
		}
		syncedCachedEnforcer.SetExpireTime(60 * 60)
		_ = syncedCachedEnforcer.LoadPolicy()
	})

	return &casbinRuleRepo{
		data:           data,
		log:            log.NewHelper(logger),
		syncedEnforcer: syncedCachedEnforcer,
	}
}

// UpdateCasbin 更新权限规则
// RoleKey = v0, Path = v1, Method = v2
func (c *casbinRuleRepo) UpdateCasbin(ctx context.Context, roleKey string, rules [][]string) error {
	if err := c.ClearCasbin(0, roleKey); err != nil {
		return err
	}
	_ = c.syncedEnforcer.LoadPolicy()
	success, err := c.syncedEnforcer.AddPolicies(rules)
	if err != nil {
		return err
	}
	if !success {
		return errors.New("存在相同api,添加失败,请联系管理员")
	}
	_ = c.syncedEnforcer.SavePolicy()
	return nil
}

// UpdateCasbinApi 更新 API 路径
func (c *casbinRuleRepo) UpdateCasbinApi(ctx context.Context, oldPath string, newPath string, oldMethod string, newMethod string) error {
	q := c.data.Query(ctx).CasbinRule
	_, err := q.WithContext(ctx).Where(q.V1.Eq(oldPath), q.V2.Eq(oldMethod)).UpdateColumns(map[string]any{
		"v1": newPath,
		"v2": newMethod,
	})
	return err
}

// GetPolicyPathByRoleId 获取角色权限路径
func (c *casbinRuleRepo) GetPolicyPathByRoleId(roleKey string) [][]string {
	e, _ := c.syncedEnforcer.GetFilteredPolicy(0, roleKey)
	return e
}

// ClearCasbin 清除权限
func (c *casbinRuleRepo) ClearCasbin(v int, p ...string) error {
	_, err := c.syncedEnforcer.RemoveFilteredPolicy(v, p...)
	return err
}

// GetModel 获取 Casbin 模型
func (c *casbinRuleRepo) GetModel() model.Model {
	return c.syncedEnforcer.GetModel()
}

// GetAdapter 获取 Casbin 适配器
func (c *casbinRuleRepo) GetAdapter() persist.Adapter {
	return c.syncedEnforcer.GetAdapter()
}
