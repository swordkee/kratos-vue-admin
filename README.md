# Kratos Vue admin v1.0

> `kratos vue admin` 简称 `KVA` 是后端基于 `Kratos 2.x + gorm + casbin`， 前端基于`vue3` 实现的`后台管理系统`，开源版本遵循 `Apache` 开源协议，企业和个人都可以根据协议自由安装使用。

## 特性

- 遵循 `RESTful API` 设计规范 & 基于接口的编程规范
- 基于 `Kratos 2.x` 框架（支持微服务架构）.
- 基于 `Casbin` 的 RBAC 访问控制模型 -- **权限控制可以细粒度到按钮 & 接口**
- 基于 `gorm` 的数据库存储
- 基于 `WIRE` 的依赖注入 -- 依赖注入本身的作用是解决了各个模块间层级依赖繁琐的初始化过程
- 基于 `Zap & Context` 实现了日志输出，通过结合 Context 实现了统一的 TraceID/UserID 等关键字段的输出(同时支持日志钩子写入到`Gorm`)
- 基于 `JWT` 的用户认证 -- 基于 JWT 的黑名单验证机制
- 基于 `Swaggo` 自动生成 `Swagger` 文档 -- 独立于接口的 mock 实现
- 基于 `net/http/httptest` 标准包实现了 API 的单元测试
- 基于 `go mod` 的依赖管理(国内源可使用：<https://goproxy.cn/>)

### 安装依赖工具

```shell
# 初始化

make init

# 生成全部代码
make all

# 下载依赖

go mod tidy

```

### 启动命令

```shell
kratos run
```

### 构建

```shell
go build -o kva
```

## DDD 四层架构

本项目遵循 **DDD (Domain-Driven Design)** 四层架构设计，实现了清晰的分层和解耦：

```
┌─────────────────────────────────────────────────────────────┐
│                         API Layer                            │
│                     (api/admin/v1/*.proto)                   │
│  - Protobuf 服务定义                                         │
│  - HTTP/gRPC 路由映射                                        │
│  - Swagger/OpenAPI 文档生成                                  │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                      Service Layer                           │
│                   (app/admin/internal/service/)              │
│  - 服务实现层：DTO ↔ 业务实体转换                             │
│  - 业务流程编排                                              │
│  - 依赖注入配置 (wire_gen.go)                                │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                       Biz Layer                              │
│                  (app/admin/internal/biz/)                   │
│  - 业务逻辑层：领域实体定义                                   │
│  - 仓储接口定义 (repo)                                       │
│  - 业务规则校验                                              │
└─────────────────────────────────────────────────────────────┘
                              ↓
┌─────────────────────────────────────────────────────────────┐
│                       Data Layer                             │
│                  (app/admin/internal/data/)                  │
│  - 数据访问层：GORM Gen 生成的 DAO/Query                     │
│  - 数据模型定义 (dal/model/)                                 │
│  - Redis 缓存操作                                            │
└─────────────────────────────────────────────────────────────┘
```

### 目录结构

```
api/admin/v1/                    # API 层 - Protobuf 定义
├── *.proto                      # 服务接口定义
├── *_pb.go                      # Protobuf 生成的 Go 代码
└── *_http.pb.go                 # HTTP 路由处理代码

app/admin/internal/
├── service/                     # Service 层 - 服务实现
│   ├── sysuser.go               # 用户服务实现
│   ├── roles.go                 # 角色服务实现
│   └── convert.go               # DTO ↔ 实体转换
├── biz/                         # Biz 层 - 业务逻辑
│   ├── biz.go                   # 业务接口定义
│   ├── sysuser.go               # 用户业务逻辑
│   └── sysrole.go               # 角色业务逻辑
└── data/                        # Data 层 - 数据访问
    ├── dal/                     # GORM Gen 生成
    │   ├── model/               # 数据模型定义
    │   └── query/               # Query 接口定义
    ├── sysuser.go               # 用户数据访问
    └── redis.go                 # Redis 操作
```

### 命名规范

#### 服务层 (Service Layer)

| 命名模式 | 说明 | 示例 |
|---------|------|------|
| `Create{Entity}` | 创建实体 | `CreateSysUser(ctx, req)` |
| `Update{Entity}` | 更新实体 | `UpdateSysUser(ctx, req)` |
| `Delete{Entity}` | 删除实体 | `DeleteSysUser(ctx, req)` |
| `Get{Entity}` | 获取单个实体 | `GetSysUser(ctx, req)` |
| `List{Entity}s` | 获取实体列表 | `ListSysUsers(ctx, req)` |

#### 业务层 (Biz Layer)

| 命名模式 | 说明 | 示例 |
|---------|------|------|
| `Create{Entity}` | 创建业务处理 | `CreateSysUser(ctx, user) error` |
| `Update{Entity}` | 更新业务处理 | `UpdateSysUser(ctx, user) error` |
| `Delete{Entity}` | 删除业务处理 | `DeleteSysUser(ctx, id) error` |
| `Find{Entity}` | 查询单个实体 | `FindSysUser(ctx, id) (*model.SysUser, error)` |
| `Query{Entity}s` | 查询实体列表 | `QuerySysUsers(ctx, query) ([]*model.SysUser, error)` |

#### 数据层 (Data Layer)

| 命名模式 | 说明 | 示例 |
|---------|------|------|
| `Create` | 插入记录 | `dao.SysUser.Create(&user)` |
| `Save` | 保存/更新记录 | `dao.SysUser.Save(&user)` |
| `Delete` | 删除记录 | `dao.SysUser.Delete().Where("id=?", id).Exec()` |
| `First` | 获取第一条 | `dao.SysUser.First(&user, where...)` |
| `Find` | 查询多条 | `dao.SysUser.Find(&users, where...)` |
| `Count` | 计数 | `dao.SysUser.Count()` |

### 代码示例

#### 1. API 层 (Protobuf 定义)

```protobuf
service SysUser {
  rpc CreateSysUser (CreateSysUserRequest) returns (SysUserReply);
  rpc UpdateSysUser (UpdateSysUserRequest) returns (SysUserReply);
  rpc DeleteSysUser (DeleteSysUserRequest) returns (DeleteSysUserReply);
  rpc GetSysUser (GetSysUserRequest) returns (SysUserReply);
  rpc ListSysUser (ListSysUserRequest) returns (ListSysUserReply);
}
```

#### 2. Service 层实现

```go
// app/admin/internal/service/sysuser.go

type SysUserService interface {
    CreateSysUser(ctx context.Context, req *v1.CreateSysUserRequest) (*v1.SysUserReply, error)
    UpdateSysUser(ctx context.Context, req *v1.UpdateSysUserRequest) (*v1.SysUserReply, error)
    DeleteSysUser(ctx context.Context, req *v1.DeleteSysUserRequest) (*v1.DeleteSysUserReply, error)
    GetSysUser(ctx context.Context, req *v1.GetSysUserRequest) (*v1.SysUserReply, error)
    ListSysUser(ctx context.Context, req *v1.ListSysUserRequest) (*v1.ListSysUserReply, error)
}

type sysUserService struct {
    biz biz.SysUserBiz
}

func NewSysUserService(b biz.SysUserBiz) *sysUserService {
    return &sysUserService{biz: b}
}

func (s *sysUserService) CreateSysUser(ctx context.Context, req *v1.CreateSysUserRequest) (*v1.SysUserReply, error) {
    // DTO → 实体转换
    user := &model.SysUser{
        Username: req.Username,
        Password: req.Password,
        Nickname: req.Nickname,
        Phone:    req.Phone,
        Email:    req.Email,
        Status:   int32(req.Status),
        DeptID:   req.DeptId,
        RoleIDs:  req.RoleIds,
    }
    
    // 调用业务层
    err := s.biz.CreateSysUser(ctx, user)
    if err != nil {
        return nil, err
    }
    
    // 实体 → Reply 转换
    return &v1.SysUserReply{
        User: convertUser(user),
    }, nil
}

func (s *sysUserService) GetSysUser(ctx context.Context, req *v1.GetSysUserRequest) (*v1.SysUserReply, error) {
    user, err := s.biz.FindSysUser(ctx, int64(req.Id))
    if err != nil {
        return nil, err
    }
    return &v1.SysUserReply{
        User: convertUser(user),
    }, nil
}
```

#### 3. Biz 层实现

```go
// app/admin/internal/biz/sysuser.go

type SysUserBiz interface {
    CreateSysUser(ctx context.Context, user *model.SysUser) error
    UpdateSysUser(ctx context.Context, user *model.SysUser) error
    DeleteSysUser(ctx context.Context, id int64) error
    FindSysUser(ctx context.Context, id int64) (*model.SysUser, error)
    QuerySysUsers(ctx context.Context, query *QuerySysUserRequest) ([]*model.SysUser, int64, error)
}

type sysUserBiz struct {
    repo data.SysUserRepo
    logger *zap.Logger
}

func NewSysUserBiz(repo data.SysUserRepo, logger *zap.Logger) *sysUserBiz {
    return &sysUserBiz{repo: repo, logger: logger}
}

func (b *sysUserBiz) CreateSysUser(ctx context.Context, user *model.SysUser) error {
    // 业务规则校验
    if user.Username == "" {
        return errors.New("用户名不能为空")
    }
    
    // 检查用户名是否已存在
    existing, _ := b.repo.FindByUsername(ctx, user.Username)
    if existing != nil {
        return errors.New("用户名已存在")
    }
    
    // 密码加密
    user.Password = hashPassword(user.Password)
    
    // 密码加密
    user.Password = hashPassword(user.Password)
    
    // 调用数据层
    return b.repo.Create(ctx, user)
}

func (b *sysUserBiz) FindSysUser(ctx context.Context, id int64) (*model.SysUser, error) {
    return b.repo.FindByID(ctx, id)
}
```

#### 4. Data 层实现

```go
// app/admin/internal/data/sysuser.go

type SysUserRepo interface {
    Create(ctx context.Context, user *model.SysUser) error
    Update(ctx context.Context, user *model.SysUser) error
    Delete(ctx context.Context, id int64) error
    FindByID(ctx context.Context, id int64) (*model.SysUser, error)
    FindByUsername(ctx context.Context, username string) (*model.SysUser, error)
    List(ctx context.Context, query *QuerySysUserRequest) ([]*model.SysUser, int64, error)
}

type sysUserRepo struct {
    data *data.Data
    logger *zap.Logger
}

func NewSysUserRepo(data *data.Data, logger *zap.Logger) *sysUserRepo {
    return &sysUserRepo{data: data, logger: logger}
}

func (r *sysUserRepo) Create(ctx context.Context, user *model.SysUser) error {
    return r.data.SysUser.WithContext(ctx).Create(user)
}

func (r *sysUserRepo) FindByID(ctx context.Context, id int64) (*model.SysUser, error) {
    user := &model.SysUser{}
    err := r.data.SysUser.WithContext(ctx).Where(r.data.SysUser.ID.Eq(id)).First(user)
    if err != nil {
        return nil, err
    }
    return user, nil
}

func (r *sysUserRepo) List(ctx context.Context, query *QuerySysUserRequest) ([]*model.SysUser, int64, error) {
    stmt := r.data.SysUser.WithContext(ctx)
    
    // 条件过滤
    if query.Username != "" {
        stmt = stmt.Where(r.data.SysUser.Username.Like("%" + query.Username + "%"))
    }
    if query.Status != 0 {
        stmt = stmt.Where(r.data.SysUser.Status.Eq(int32(query.Status)))
    }
    
    // 分页
    offset := (query.Page - 1) * query.PageSize
    
    // 查询列表
    users := make([]*model.SysUser, 0)
    err := stmt.Offset(offset).Limit(query.PageSize).Find(&users).Error
    if err != nil {
        return nil, 0, err
    }
    
    // 统计总数
    total, err := stmt.Count()
    if err != nil {
        return nil, 0, err
    }
    
    return users, total, nil
}
```

### 最佳实践

1. **接口定义**：在 Biz 层定义接口，Service 层依赖接口而非具体实现
2. **依赖注入**：使用 Wire 进行依赖注入配置 (`wire_gen.go`)
3. **错误处理**：统一使用 `pkg/errs` 包的错误码体系
4. **日志记录**：使用 `zap.Logger`，通过 Context 传递 TraceID
5. **事务管理**：使用 GORM 的 `Transaction` 方法
6. **缓存策略**：热点数据使用 Redis 缓存，注意缓存一致性

## 特别鸣谢

- `kratos` 微服务框架。
- `vue3` 使用该前端框架进行开发后台管理web 界面。
