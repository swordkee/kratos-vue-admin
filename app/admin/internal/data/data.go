package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
	go_redis "github.com/redis/go-redis/v9"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/biz"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/conf"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/admin"
	"github.com/swordkee/kratos-vue-admin/app/admin/internal/data/gen/dao"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// ProviderSet is data providers.
var ProviderSet = wire.NewSet(
	NewData,
	NewDB,
	NewTransaction,
	NewRedis,
	NewRedisRepo,
	admin.NewSysUserRepo,
	admin.NewSysMenuRepo,
	admin.NewSysDeptRepo,
	admin.NewSysPostRepo,
	admin.NewSysApiRepo,
	admin.NewSysRoleRepo,
	admin.NewSysRoleMenuRepo,
	admin.NewCasbinRuleRepo,
	admin.NewSysDictDataRepo,
	admin.NewSysDictTypeRepo,
)

// Data .
type Data struct {
	log   *log.Helper
	query *dao.Query
	db    *gorm.DB
	rdb   go_redis.UniversalClient
}

// contextTxKey 用于在 context 中传递 GORM Gen 的事务 Query
type contextTxKey struct{}

func toGormLogLevel(d conf.GormLogLevel) gormLogger.LogLevel {
	switch d {
	case conf.GormLogLevel_silent:
		return gormLogger.Silent
	case conf.GormLogLevel_error:
		return gormLogger.Error
	case conf.GormLogLevel_warn:
		return gormLogger.Warn
	case conf.GormLogLevel_info:
		return gormLogger.Info
	default:
		return gormLogger.Warn
	}
}

func NewDB(config *conf.Data, logger log.Logger) *gorm.DB {
	logs := log.NewHelper(log.With(logger, "module", "receive-service/data/gorm"))

	db, err := gorm.Open(mysql.Open(config.Database.Source), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   gormLogger.Default.LogMode(toGormLogLevel(config.Database.LogLevel)),
	})
	if err != nil {
		logs.Fatalf("failed opening connection to mysql: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logs.Fatalf("failed get sql.DB : %v", err)
	}
	sqlDB.SetMaxIdleConns(int(config.Database.MaxIdleConns))
	sqlDB.SetMaxOpenConns(int(config.Database.MaxOpenConns))

	return db
}

// NewData .
func NewData(db *gorm.DB, logger log.Logger, rdb go_redis.UniversalClient) (*Data, func(), error) {
	logs := log.NewHelper(log.With(logger, "module", "receive-service/data"))
	d := &Data{
		log:   logs,
		query: dao.Use(db),
		db:    db,
		rdb:   rdb,
	}
	return d, func() {}, nil
}

func NewTransaction(d *Data) biz.Transaction {
	return d
}

// Transaction 使用 GORM Gen 自带的事务方式，通过 context 传递带事务的 tx (dao.Query)
// 这样所有 repo 操作都会自动使用同一个事务，实现原子性操作
func (d *Data) Transaction(ctx context.Context, fn func(ctx context.Context) error) error {
	// 使用 GORM Gen 的事务模式，通过 WithContext 传递事务上下文
	return d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建带事务的 dao.Query
		txQuery := dao.Use(tx)
		// 将带事务的 Query 存入 context，供后续 repo 操作使用
		ctx = context.WithValue(ctx, contextTxKey{}, txQuery)
		return fn(ctx)
	})
}

func (d *Data) Query(ctx context.Context) *dao.Query {
	tx, ok := ctx.Value(contextTxKey{}).(*dao.Query)
	if ok {
		return tx
	}
	return d.query
}

func convertPageSize(page, size int32) (limit, offset int) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 10
	}
	limit = int(size)
	offset = int((page - 1) * size)
	return
}

func buildLikeValue(key string) string {
	return fmt.Sprintf("%%%s%%", key)
}

func NewRedis(conf *conf.Data) go_redis.UniversalClient {
	var redisClient go_redis.UniversalClient
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	redisClient = go_redis.NewClient(&go_redis.Options{
		Addr:     conf.Redis.Addr,
		Username: conf.Redis.Username,
		Password: conf.Redis.Password,      // no password set
		DB:       int(conf.Redis.Database), // use default DB
		PoolSize: 100,                      // 连接池大小
	})
	_, err := redisClient.Ping(ctx).Result()
	if err != nil {
		panic(err)
	}
	return redisClient
}
