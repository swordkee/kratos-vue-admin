# AGENTS.md（仓库级 AI 协作规则）

## 仓库定位

本仓是 **KVA 公共模板（上游）**。下游产品线仓（均名 `go-kva-admin`）：

| 仓 | 路径 | 集成分支 | 业务 |
|---|---|---|---|
| 商城 | `E:\work\mall\go-kva-admin` | dev（main 对齐） | 商城后台（双库 124+274 表） |
| 广告 | `E:\work\third-party\go-kva-admin` | dev（main 对齐） | 流量投放/ads 后台（表名 kva_*） |
| 壁纸 | `E:\work\wallpaper\go-kva-admin` | dev（main 对齐） | 壁纸后台（proto 已分域） |
| exAdmin | `E:\work\bmall\go-exAdmin` | dev | 成熟业务仓，KVA 层修复常源自它 |
| payAdmin | `E:\work\bmall\history\go-payAdmin` | dev | 支付后台 |

**双向同步纪律**：通用能力（安全防线/登录体系/中间件/前端 request 层）改动应评估回灌模板；模板与下游同层文件保持同构（module/import 前缀是唯一允许的差异）。业务域代码（支付/广告/壁纸内容）互不同步。

## worktree 目录规则（适用于本仓所有改动）

- 改动一律在**独立 worktree**（`.worktrees/<name>` + 独立分支，从 `origin/dev`（模板为 `origin/main`）起）内进行，主检出目录保持干净；集成后 `git worktree remove --force` + `git branch -D` + `git worktree prune`。
- 合回用**快进推送** `git push origin <sha>:refs/heads/dev`（在 worktree 内直接推，不碰主检出工作区）；本地 ff-only 合回仅在主检出干净时可用。
- 合并前先 `git fetch` + `git rebase origin/<分支>` 对齐远端（同事活跃，随时可能前移）。

## 子代理派工口径

- 工单必须给：worktree **绝对路径** + 分支名 + BASE 提交；明说「你不了解历史，信息全在下面」并附完整背景与已确证事实。
- 子代理允许在指定 worktree 内 commit，**禁 push/merge**；父会话复核后统一推送。
- 验收必须要求「红→绿证据」（先让回归测试打出红，再修复转绿）或变异自检，不接受只报「测试全过」。
- 假活判据：worktree 长时间零写入 + 无 go/node 进程；接管前先 `git status` 盘点已产出内容，别推倒重来。

## 本仓踩坑记录（改代码前扫一遍）

- **kratos http `ctx.Header()` 返回的是请求头**，对它 Set 是静默丢失；设响应头必须 `ctx.Response().Header()`。
- **`StrictSlash(true)`（kratos 默认）下 catch-all `/{path:.*}` 会把 `GET /` 变 301 自环**，须显式注册精确 `GET /`（web embed 实证）。
- **`*.pb.go` 禁止文本替换**：rawDesc 是字节编码，改字符串不改长度/偏移会运行时炸；一律重新 protoc。
- **MySQL 8.4 无 `ADD COLUMN IF NOT EXISTS`**（MariaDB 语法，直接 1064）：幂等 DDL 用 `information_schema.COLUMNS` 判定 + `PREPARE/EXECUTE`。
- **proto 字段号只增不复用**：加字段前 grep 本仓 proto 实际占位（各仓布局不同：kv/gva/wp/模板=26，exAdmin=27 因 memberId 占 26）。
- **`go test` 一律 `-p 1` 串行**（提交内存小，并行会 OOM 假红）；worktree 内 GOCACHE 若报「path not found」多为依赖未下载完的假错，重跑即可。
- **GORM `Updates(struct)` 跳过零值**：置 0/空串字段须用 `UpdateColumn`/map 形态（登录方式切回 TOTP 场景实证）。
- **中间态 token 必须互斥**：MfaPending/SmsPending 各只认自己的校验通道，跨通道一律拒（防串道绕过）。
- **IP 黑名单依赖真实 IP**：经 nginx 部署必须透传 `X-Forwarded-For`，否则全部判成代理机 IP。
- **wire 在部分 BASE 上跑不动**（缺 provider 报错与本批无关）：改为手工同步 `wire_gen.go` 最小 diff，别硬修别人域的装配。
- **前端 `npm ci` 会因 lock 与 package.json 不同步失败**（既有问题）：用 `npm install`，**不要提交 lock 变化**。
- worktree 复用前端：`cmd /c "mklink /J web\node_modules <主检出>\web\node_modules"`，用完 `rmdir`（junction 不是目录，勿递归删）。

## 生成物与代码生成

- protoc：`D:\Dev\Scoop\apps\protobuf\current\bin\protoc.exe`；插件在 `D:\Dev\goPath\bin`（go/go-grpc/go-http/validate/openapi）。
- conf 结构、DAO/model（gorm/gen）、wire 各自入口见 Makefile；`make all` 为全量。
- 重生成后必须 `git diff` 检查**无关漂移**（如 gorm-gen 表名单复数漂移、openapi 全量重排），漂移回退。

## 质量门禁

- 后端：`go build ./...` + `go test -p 1 -count=1 ./...` 全绿是合入门槛。
- 前端：`npx vite build` rc=0。
- 状态/权限类修复配**三把锁**：异常态拒绝、豁免格正向锁、豁免不外溢反向锁。
- 间歇性缺陷（加密/会话类）连跑 3 次取稳。
