# Kratos Vue admin v2.0

> `kratos vue admin` 简称 `KVA` 是后端基于 `Kratos 3.x + gorm + casbin`、前端基于 `Vue3 + Element Plus + TypeScript` 实现的后台管理系统框架。本仓是 **KVA 公共模板（上游）**：商城/广告/壁纸等多条产品线的管理后台均由本模板 fork 派生，通用能力在模板与下游仓之间双向同步。开源版本遵循 `Apache` 开源协议。

## 特性

### 框架与架构
- 遵循 `RESTful API` 设计规范 & 基于接口的编程规范
- 基于 `Kratos 3.0` 框架（支持微服务架构），proto 单一事实源：HTTP 路由/gRPC/OpenAPI 全部由 `api/**/*.proto` 生成
- 基于 `Casbin` 的 RBAC 访问控制模型 —— **权限控制可细粒度到按钮 & 接口**
- 基于 `gorm` + `gorm/gen` 的数据库存储（DAO/model 生成物）
- 基于 `Wire` 的依赖注入
- 基于 `logx + Context` 的统一日志（TraceID/UserID 自动携带，日志钩子写入 Gorm）
- 基于 `go mod` 的依赖管理（国内源可使用：<https://goproxy.cn/>）

### 安全体系（多产品线下打磨回灌的通用防线）
- **JWT ES384**：登录 token 为 ECDSA P-384 签名（ES384），私钥不落仓库、由配置/环境注入；支持 JWT 黑名单与 IP 黑名单（经代理时须透传 `X-Forwarded-For`）
- **登录方式二选一（login_method）**：建号时指定 `1=TOTP / 2=手机验证码`，之后强制按该方式登录；TOTP 未绑定用户无条件引导绑定（MustEnroll）；短信用户密码通过后发码二次校验（`POST /system/user/phone/verify`）；TOTP/短信两种中间态 token 跨通道互拒；短信通道未配置时 fail-closed 拒绝
- **TOTP 双因素**：绑定/校验/解绑全流程 + 登录图形验证码（真实校验，可开关）
- **登录失败锁定（loginlock）**：阈值/窗口/锁定时长后台可配（sys_security_config），Redis 共享 / 无 Redis 进程内存双后端，nil 安全装配
- **敏感信息权限可见性（sensitive）**：列表默认脱敏（手机号/身份证等），`/sensitive/view` 虚拟权限点控制明文查看，权限判定故障 fail-closed；日志脱敏统一由 `pkg/sensitive` 输出掩码
- **媒体库（附件分类 / 文件上传下载 / 断点续传）**：分类树增删改、分页文件库（改名/单删/批量删/导入外链）、大文件 1MB 分片上传（分片摘要 + 整文件摘要双重校验、秒传、续传、合并建档）、本地/阿里云 OSS 双模存储、`/static/*` 本地静态面（路径穿越 fail-closed）；前端三页 `web/src/views/media/{upload,file,scanUpload}`
- **操作审计**：全接口操作记录（record_log），敏感查询行为留痕
- **CORS 白名单配置化**（未配置即拒绝，fail-closed）；pprof 生产环境自动关闭

### 部署
- **单二进制部署**：前端构建产物经 `//go:embed` 打进后端（`app/admin/internal/server/webdist`），一个进程同时伺服管理端页面与 API（hash 路由，无 history fallback）
- 三库分组（可选）：default / kva / trade 多 MySQL 组配置样例见 `app/admin/configs/examples/`

## 快速开始

```shell
# 安装依赖工具（kratos CLI / protoc 插件 / wire）
make init

# 生成全部代码（proto → pb/http/grpc、conf、wire）
make all

go mod tidy

# 本地开发启动
kratos run            # 或 make build_server 后运行二进制
```

### 构建与发布

```shell
# 常规后端二进制
make build_server

# 一键发布产物：前端 build → 同步进 embed dist → 后端单二进制 → upx 压缩
make release
```

Linux 交叉编译单包（Windows 开发机产出线上包）：

```shell
make web                                   # 前端构建并同步 embed
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -ldflags "-s -w" -o bin/admin-linux-amd64 \
  app/admin/cmd/server/main.go app/admin/cmd/server/wire_gen.go
upx --best --lzma bin/admin-linux-amd64    # 可选，约 76MB → 19MB
```

运行：`./admin-linux-amd64 -conf ./configs/`（默认 HTTP 监听 `0.0.0.0:8001`，见 `app/admin/configs/config.yaml`）。

### Nginx 参考配置（单包形态）

前端与 API 同端口同进程，nginx 纯反代即可；**必须透传真实 IP**（登录 IP 黑名单与审计依赖它）：

```nginx
upstream kva_admin { server 127.0.0.1:8001; keepalive 32; }

server {
    listen 80;
    server_name admin.example.com;
    client_max_body_size 50m;

    location / {
        proxy_pass http://kva_admin;
        proxy_http_version 1.1;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Real-IP       $remote_addr;
        proxy_set_header Host            $host;
        proxy_set_header Connection      "";
        proxy_read_timeout 60s;
    }

    # embed 静态产物带 hash 文件名，可放心长缓存
    location ~* \.(js|css|png|jpg|svg|woff2?)$ {
        proxy_pass http://kva_admin;
        expires 7d;
        add_header Cache-Control "public, immutable";
    }
}
```

> 前端为 hash 路由（`/#/xxx`），无需 `try_files` history fallback；API 与页面共用 `/`，无需按路径拆分 location。

### 数据库初始化

- 基线 DDL+种子：`deploy/sql/kva.sql`
- 增量 DDL 一律幂等写法落 `deploy/sql/`（⚠️ MySQL 8.4 不支持 `ADD COLUMN IF NOT EXISTS`，用 `information_schema` + `PREPARE` 判定）

## DDD 四层架构

本项目遵循 **DDD (Domain-Driven Design)** 四层架构设计，实现了清晰的分层和解耦：

```
┌─────────────────────────────────────────────────────────────┐
│                         API Layer                            │
│                     (api/admin/v1/*.proto + api/media/v1)     │
│  - Protobuf 服务定义                                         │
│  - HTTP/gRPC 路由映射                                        │
│  - OpenAPI 文档生成                                          │
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
│  - 数据访问层：GORM Gen 生成的 DAO/Model                      │
│  - Redis 缓存操作                                            │
└─────────────────────────────────────────────────────────────┘
```

### 目录结构

```
api/admin/v1/                    # API 层 - Protobuf 定义（系统面：用户/角色/菜单/字典/日志…）
api/media/v1/                    # API 层 - 媒体库域（分类/文件/断点续传；分域首例）
app/admin/
├── cmd/server/                  # 入口 + wire_gen.go
├── configs/                     # config.yaml / examples（多库分组样例）
└── internal/
    ├── server/                  # HTTP/gRPC 装配、中间件链、webdist(embed 前端)
    ├── service/                 # Service 层 - DTO ↔ 实体转换
    ├── biz/                     # Biz 层 - 领域逻辑 + 仓储接口
    ├── data/                    # Data 层 - gorm/gen DAO、Redis
    └── pkg/                     # 仓内私有库（authz/mfa/sensitive/...）
pkg/                             # 跨仓可复用库（loginlock/log/...）
deploy/sql/                      # 基线 kva.sql + 幂等增量 DDL
web/                             # Vue3 前端（构建产物同步进 server/webdist）
scripts/                         # 代码生成/校验脚本
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
| `Find{Entity}` | 查询单个 | `FindSysUser(ctx, id) (*model.SysUser, error)` |
| `Query{Entity}s` | 查询列表 | `QuerySysUsers(ctx, query) ([]*model.SysUser, error)` |

#### 数据层 (Data Layer)

| 命名模式 | 说明 | 示例 |
|---------|------|------|
| `Create` | 插入记录 | `dao.SysUser.WithContext(ctx).Create(user)` |
| `Save` | 保存/更新 | `dao.SysUser.WithContext(ctx).Save(user)` |
| `Delete` | 删除记录 | `dao.SysUser.WithContext(ctx).Delete()` |
| `First` | 获取第一条 | `dao.SysUser.WithContext(ctx).FirstByID(id)` |
| `Find` | 查询多条 | `dao.SysUser.WithContext(ctx).Find()` |
| `Count` | 计数 | `dao.SysUser.WithContext(ctx).Count()` |

### 最佳实践

1. **接口定义**：仓储接口定义在 Biz 层，Data 层实现，Service 层依赖 Biz 接口
2. **依赖注入**：统一走 Wire（`ProviderSet`，新增 service/biz/data 记得注册）
3. **错误处理**：统一 `pb.ErrorXxx` 生成的错误码体系（proto errors 枚举），不回显内部错误原文
4. **生成物纪律**：`*.pb.go` 禁止文本替换（rawDesc 字节数会变），一律重新 protoc；`wire_gen.go` 用 wire 重生成
5. **日志**：`logx` ctx-first（`log.Infof(ctx, ...)`），敏感字段经 `pkg/sensitive` 掩码
6. **事务**：使用 GORM 的 `Transaction` 方法
7. **中间件顺序**：Auth → MFA 守卫 → casbin → 操作记录 → sensitive guard；catch-all 静态路由注册在全部 API 之后（有顺序回归锁看护）

## 特别鸣谢

- `kratos` 微服务框架。
- `vue3` 使用该前端框架进行开发后台管理 web 界面。
