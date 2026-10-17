---
description: AKIOI（信奥在线评测系统）项目规则：架构、部署、数据库、配置、日志、判题、代码与 Git 规范
globs:
alwaysApply: true
---

# AKIOI 项目 Rule

信息学奥赛（OI）在线评测系统：题库 + 提交判题 + 比赛排名。技术栈：Go1.20+/Gin、Vue3+Vite、MySQL8.0、Redis7.0、Docker Compose（含判题沙箱）。与 StarLoft / FancyWall 共用后端实现风格。

系统按部署边界拆成两个可独立部署的单元：`main/`（主站：MySQL、Redis、后端 API、前端）与 `judgeserver/`（评测节点，可多机横向扩展）。

## 1. 架构与目录

- `main/backend/`（Go module `akioi`）：入口 `cmd/server/main.go`（API 服务）、`cmd/akioi-cli/main.go`（运维 CLI）；业务包统一 `internal/`（禁止根目录平铺），包名单数短名（`handler`/`model`/`router`），路由独立 `internal/router` 包，`Setup(...)` 返回 `*gin.Engine`。
- `main/frontend/`（Vue3+Vite）：单一 SPA，用户站与管理后台按路由前缀与布局区分；UI 遵循简洁风格，按钮文案精简。
- `judgeserver/backend/`（Go module 同为 `akioi`）：入口 `cmd/judge/main.go`（评测进程）；只含评测所需的 `internal/{config,logger,database,redis,queue,judge,model}`，不引入 handler/router/httpx/migrate，评测节点不得改表结构。
- API 与评测进程分离部署：`server` 通过 Redis 列表队列下发判题任务，`judge` 阻塞拉取任务并直接回写数据库；两者不通过 HTTP 互相调用。
- **两侧共享契约**（改动必须同步到另一侧）：`internal/model/*`（表结构与常量）、`internal/queue.Task`（队列消息格式）、`.env` 的 `JUDGE_QUEUE_KEY`。
- 路由一级段约定：`/api/v1/*`（用户侧 JSON API）、`/api/admin/v1/*`（管理后台，需管理员角色）、`/healthz` 与 `/readyz`（探针）、`/uploads/*`（静态文件）。前端 SPA 的后台页面走前端路由 `/admin/*`，**不反代给后端**（因此后台接口前缀必须是 `/api/admin/v1`，避免与前端路由冲突）。
- 评测节点另有控制面接口，不属站内 API、不经主站反代：`PUT|DELETE /internal/v1/testdata/{problemID}`，用 `X-Sync-Secret` 共享密钥校验（常量时间比较），只在 `JUDGE_LISTEN_ADDR` 上监听，仅对主站开放。
- 修改代码后保持 `gofmt` 干净；提交前在 `main/backend` 与 `judgeserver/backend` 两个 module 下分别确认 `go build ./...`、`go vet ./...`、`go test ./...` 通过。

## 2. 部署与网络

- `main/docker-compose.yml` 编排 `mysql`、`redis`、`server`、`frontend`（Nginx 容器，内置前端构建产物 + SPA 回退，反代 `/api`、`/uploads` 至后端）；`judgeserver/docker-compose.yml` 编排 `judge` 与只在 `sandbox` profile 下按需构建的 `sandbox`（产出 `akioi-sandbox:latest` 镜像）。首次部署：主站 `cd main && docker compose up -d --build`；评测节点 `cd judgeserver && docker compose --profile sandbox build sandbox && docker compose up -d --build`。
- 构建文件与被构建的目录放在一起，不设单独的 `deploy/`：`main/backend/Dockerfile`（server + akioi-cli）、`main/frontend/Dockerfile` 与 `main/frontend/nginx.conf`、`judgeserver/backend/Dockerfile`（judge）、`judgeserver/sandbox/Dockerfile`（沙箱镜像）。compose 的 `build.context` 指向对应子目录（`./backend`、`./frontend`、`./sandbox`），因此 Dockerfile 内的 COPY 路径以该子目录为根。
- 前端构建上下文是 `frontend/`，`frontend/.dockerignore` 必须排除 `node_modules` 与 `dist`，否则本机的 `node_modules`（尤其 Windows 上安装的 esbuild/rollup 二进制）会进入镜像干扰构建。
- 后端仅在容器内网暴露 `8080`（`expose`，不映射宿主端口）；对外 TLS 终止与域名分发由宿主机/1Panel Nginx 反代到前端容器端口，需透传 `Host` 头与真实 IP（`X-Real-IP`/`X-Forwarded-For`）。
- 反代到后端必须用「变量 + 无 URI 的 proxy_pass」：`proxy_pass http://$backend_endpoint;`（在 `proxy_pass` 里写 URI 会截断剩余路径）；变量与 `resolver 127.0.0.11` 必须保留（运行期解析 compose 服务名，避免 DNS 未就绪启动失败）。
- **判题通过 docker-outside-of-docker 实现**：`judge` 挂载 `/var/run/docker.sock` 与本机同路径的 `judgeserver/data/testdata`、`judgeserver/data/work`，用 `docker run` 创建一次性沙箱容器；因此 `HOST_TESTDATA_DIR`/`HOST_WORK_DIR` 必须配成本机绝对路径，否则沙箱挂载的是不存在的路径。`docker.sock` 只给 `judge`，不给 `server`。
- **跨机分布式部署**：评测节点通过 `.env` 的 `DB_HOST`/`REDIS_ADDR` 连接主站；与主站同机部署填 `host.docker.internal`，跨机填主站内网 IP。主站的 MySQL/Redis 默认只绑 `127.0.0.1`（`DB_BIND_ADDR`/`REDIS_BIND_ADDR`），跨机部署时改为内网 IP 并用防火墙把来源限制为评测机，禁止暴露到公网。
- **测试数据分发**：测试数据落文件系统，主站改动后按 `JUDGE_NODE_ENDPOINTS` 的顺序**依次**推送到各评测节点，单个节点失败只记录并继续，接口响应返回 `sync_failed` 节点列表供前端提示；推送把 zip 打到临时文件后流式上传，不整包进内存。节点侧先写临时目录再改名替换，避免留下半个数据包；节点未配置 `JUDGE_SYNC_SECRET` 时不监听接收接口。
- 数据持久化到各自目录下的 `data/`：主站 `main/data/`（`mysql`、`redis`、`logs`、`testdata`、`uploads`），评测节点 `judgeserver/data/`（`logs`、`testdata`、`work`），全部 gitignore；容器以 root 运行，bind mount 目录由 Docker 自动创建即可写。

## 3. 数据库

- MySQL 8.0 单库 `akioi`，由容器 `MYSQL_DATABASE` 创建；表结构由后端启动时 GORM `AutoMigrate` 补齐；默认管理员等种子数据由启动时 `seedData()` 幂等写入（仅空表时插入）。
- 迁移统一使用 golang-migrate 版本化 SQL 文件（`main/backend/migrations/{序号}_{名称}.up.sql` / `.down.sql` + `//go:embed`），`internal/migrate` 提供 `Run(dsn)`/`Pending(dsn)`，启动时在 AutoMigrate 之前自动执行未应用版本，记录于 `schema_migrations`；迁移 DSN 由 `database.MigrateDSN(cfg)` 构造并带 `multiStatements=true`。迁移与 AutoMigrate 只属于主站，评测节点不执行任何 DDL。
- **迁移文件内禁止 `USE` 切换会话库**（会破坏 golang-migrate 的版本记账，表现为 `Dirty database version` 卡死）；跨库 DDL 一律用反引号全限定库名.表名。存量数据修复写入迁移 SQL。
- 数据访问统一 GORM，业务复杂可分 `repository`/`service` 分层，handler 不直接拼 SQL；模型带 `gorm` tag，复杂表名用 `TableName()`。
- 拼接动态库名/表名必须用 Go 反引号字符串拼接（`` ` + model.SysDB + ` ``），禁止写成双引号字符串内的字面占位符（曾致 `Error 1102`）。
- **测试数据（测试点 in/out、checker、spj）落文件系统**（主站 `main/data/testdata/{problemID}/`），库内只存元数据（测试点数量、各点分值、数据包 sha256、数据大小）；不得把测试点内容存进数据库大字段。
- 用户提交源码存 `submission.code`（`mediumtext`），不单独落文件；判题产生的临时目录与产物用完即删。
- 结果类表（`submission`）只追加，不就地覆盖历史成绩；重测必须换新的 `judge_token` 并重置状态，旧任务的结果因 token 不匹配而丢弃。

## 4. 配置（.env）

- 配置分两份：主站 `main/.env`（数据库、Redis、JWT、限流阈值、队列键、测试数据目录）与评测节点 `judgeserver/.env`（数据库与 Redis 地址、沙箱参数、编译选项、单节点并发度），各自按同目录 `.env.example` 复制生成，改后重启对应容器生效；禁止在线配置表。
- 两侧各自的 `config.Load() (*Config, error)` 读环境变量（可先 `_ = godotenv.Load()` 不覆盖已注入变量），统一用 `getEnv`/`getEnvInt` 辅助函数，不手写 `if os.Getenv(...) != ""`；评测节点的 `Config` 只保留连接主站与评测所需的字段，`validate()` 不校验主站专属项。关键密钥启动时校验；库名写死为 `config.DBName` 常量，不在 `.env` 中配置。
- 默认管理员由启动时 `Seed()` 幂等创建（仅在无任何管理员时插入），用户名与初始密码取自 `ADMIN_INIT_USERNAME`/`ADMIN_INIT_PASSWORD`；上线后必须立即改密码，不得把密码打印到日志。
- 回调/回跳地址硬编码进代码（与业务深度耦合，改动需改码）。
- **`.env` 与同目录的 `.env.example` 结构必须完全一致**：注释、分组标题、键名、键顺序逐行对齐，仅取值可不同。`.env.example` 按逻辑顺序分组并用连续序号编号。
- 每次修改 `.env.example` 后必须同步改同目录的 `.env`：保留已配置的值（含密钥、PEM 等多行 `\n` 字面量原样不动），删除已废弃键，新增键按 `.env.example` 补齐；同步后逐行结构（把所有 `KEY=值` 归一为 `KEY=` 后比较）应完全相同。
- `.env` 含真实密钥，只保留在本地/服务器，禁止提交仓库。两侧 `.env` 中的 `DB_PASSWORD`/`REDIS_PASSWORD`/`JUDGE_QUEUE_KEY`/`JUDGE_SYNC_SECRET` 必须一致。

## 5. 日志

- 不落数据库。主站写 `main/data/logs`、评测节点写 `judgeserver/data/logs`（容器内均为 `/app/logs`）：主站产生 `access.log`(HTTP)、`admin.log`(管理操作)、`business.log`(标准)、`error.log`(错误)，评测节点产生 `judge.log`(评测过程)、`business.log`、`error.log`。
- **所有类别同时写文件与控制台**（business/access/admin/judge 输出 stdout，error 输出 stderr），保证 `docker logs` 能看到全部日志与启动失败原因。
- 文件满 100MB 轮转，历史备份带递增序号永久保留，不自动删除。
- 记录操作日志时带上用户/管理员身份与 IP（游客可略 IP）；**评测日志不得整段打印用户源码**，只记语言、长度、编译错误摘要与首行输出差异。

## 6. 判题

- 用户代码一律视为不可信：评测必须在隔离容器内执行，**禁网**（`--network none`）、根文件系统只读（`--read-only`）、丢弃全部能力（`--cap-drop ALL`）、限制进程数（`--pids-limit`）、`/tmp` 用 tmpfs。
- 工作目录（`/work`）由 judge 按提交创建独立子目录并 bind mount 进沙箱：编译产物与输出文件要靠它取回宿主，因此不能用 tmpfs；每批评测结束后立即删除该目录。输出大小用 `--ulimit fsize` 在内核层兜底，另按 `OUTPUT_LIMIT_MB` 判 `OutputLimitExceeded`。
- 资源限制用 `--memory`/`--memory-swap`/`--cpus` 硬限制，并用 wall-clock 超时兜底；内存与耗时以沙箱内 `/usr/bin/time -v` 的 `Maximum resident set size` 与 `User time` 为准（沙箱镜像必须装 `time` 包）。单测试点独立执行、独立计时，超时/超内存判 `TLE`/`MLE`，不因异常退出污染其它测试点。
- 判题结果状态固定为：`Pending`、`Judging`、`Accepted`、`WrongAnswer`、`TimeLimitExceeded`、`MemoryLimitExceeded`、`RuntimeError`、`CompileError`、`SystemError`、`OutputLimitExceeded`。OI 赛制按测试点累加得分；`CompileError` 得 0 分且不执行任何测试点。
- 比对模式固定三种：`default`（忽略行尾空白与末尾空行）、`exact`（仅忽略文件末尾换行差异）、`float`（按 token 比较，数字用误差判定）。**WA 的提示信息不得包含标准答案内容**，只允许给出行号/token 位置。
- 评测任务通过 Redis 列表队列下发，**必须幂等**：任务带唯一 `judge_token`，回写时用 `WHERE id = ? AND judge_token = ?` 比对；judge 启动时重新投递 24 小时内 `Pending`/`Judging` 的提交，崩溃可自愈。
- 编译参数、编译器路径、沙箱镜像、并发度统一在 `judgeserver/.env` 配置，不得散落在代码中；语言白名单在 `judgeserver/backend/internal/judge/lang.go` 的 `ResolveLanguage` 登记编译与运行方式，并在两侧 `internal/model/constants.go` 的 `SupportedLanguages()` 中登记对前端暴露的键与名称，新增语言必须两侧同步。
- 评测机磁盘与 CPU 是共享资源：并发评测数受 `JUDGE_CONCURRENCY` 限制，队列积压时不得无限创建容器；`JUDGE_CONCURRENCY` 是单节点并发，总并发为节点数之和。
- **多评测节点去重**：`RequeueUnfinished` 会在 judge 启动时全量重投 24 小时内未完成的提交，多节点会重复投递；`Engine.Judge` 执行前用 Redis 锁 `akioi:judge:lock:{提交ID}:{judge_token}`（TTL = `JUDGE_TASK_TIMEOUT_SEC` + 60s）抢占，抢不到直接跳过，释放时用 Lua 校验锁值以免误删超时后被他节点抢占的锁。锁键含 token，因此重测（新 token）不会被旧任务的锁挡住。
- 题目通过人数 `accepted_count` 由 `Engine.markAccepted` 用单条 SQL 重算（`COUNT(DISTINCT user_id)`），重测与多节点并发都不会重复计数。

## 7. 权限与赛制

- 角色：`admin`（全部管理权限）、`user`（选手）。权限校验在中间件完成，handler 内不重复判断角色字符串。
- 账号注册必须验证手机号短信验证码并设置密码；登录只接受手机号或用户名 + 密码以及微信扫码，不提供短信验证码登录。短信验证码仅允许 `register` 与 `reset_password` 两种用途。
- 用户公开资料不得返回手机号；手机号只在本人资料与管理员用户列表中展示。
- 普通用户只能看到已发布题目；测试数据仅管理员可见，源码仅本人或管理员可见（比赛进行中按比赛可见性规则限制）。
- 比赛进行中禁止用户看到他人提交结果详情与测试点数据；榜单为 OI 赛制（同题取多次提交的最高分累加，先比总分、再比通过题数、再比最后提交时间），榜单计算唯一实现在 `internal/service/rank.go`，handler 只负责取数与权限裁剪。
- 全站积分榜只统计公开且非比赛专用题目，单题积分为 `题目积分 × 历史最高评测得分 / 测试点总分` 并封顶；排序依次比较总积分、满分题数、用户 ID。
- 题解、讨论与评论公开可读，登录后可发布；内容仅作者或管理员可编辑、删除。
- 题面须包含：题目描述、输入格式、输出格式、数据范围与说明、样例；题目有唯一题号（`P{四位数字}`）。
- 开放接口若存在，必须与站内共用同一鉴权与限流中间件，不得另起一套。

## 8. 代码风格

- 注释只解释代码客观行为；不要教程/示例式注释。移除纯测试功能与被注释掉的死代码（如跳过沙箱的判题开关、打印测试数据到日志）。
- 响应格式统一由 `internal/httpx` 输出 JSON `{code, message, data}`，`code == 0` 成功；HTTP 状态码与业务语义对齐；字段名与前端契约绑定，改动需同步前端。统一使用 `httpx.OK/Fail/BadRequest/NotFound/Conflict/ServerError` 与 `httpx.CurrentUser/MustUser/Pagination/List/PathID`。
- 中间件为返回 `gin.HandlerFunc` 的工厂函数，依赖经参数注入；限流基于 Redis 滑动窗口（Lua 脚本），管理员豁免，Redis 不可用 fail-closed。
- 主站启动顺序（`main/backend/cmd/server/main.go`）：`config.Load()` → `logger.Init` → `migrate.Run` → `database.Connect` → `AutoMigrate` → `database.Seed` → `redis.Connect` → `router.Setup(handler.New(db, cfg, rdb), cfg, db, rdb)` → `r.Run(addr)`。
- 评测节点启动顺序（`judgeserver/backend/cmd/judge/main.go`）：`config.Load()` → `logger.Init` → `database.Connect` → `redis.Connect` → 沙箱自检 → `judge.RequeueUnfinished` → 配置了 `JUDGE_SYNC_SECRET` 时启动测试数据接收接口 → 按 `JUDGE_CONCURRENCY` 起多 worker 阻塞消费队列。

## 9. Git 工作流

- 每次代码修改后自行 `git commit` 并 `git push origin/main`（用户已明确要求自动 push）。
- 远端为 `https://github.com/fancyun/akioi.git`，主分支 `main`。推送必须走本机代理：仓库内已配置 `http.proxy`/`https.proxy = http://127.0.0.1:7890`；代理未启动时直连 github.com:443 也会超时，报 `Failed to connect to github.com:443` 时先确认代理已运行再重试。
- push 时直接提交项目内全部改动（`git add -A` 全量暂存），无需只挑选本会话修改的文件。
- 仓库只保留源码；编译产物、生成目录、日志、`main/data`、`judgeserver/data`、`.env`、测试数据一律 gitignore / 删除。

## 10. 线上排查

- 代码运行在用户侧服务器上，Agent 无法直连服务器、容器或宿主机文件系统，也拿不到服务器上的 `.env` 与运行时状态。
- 需要查看线上运行状态（容器日志、`main/data/logs/*.log`、`judgeserver/data/logs/*.log`、容器内 Nginx 配置与文件、数据库数据、环境变量等）时，必须给出可直接复制执行的命令（`docker exec` / `docker logs` / `grep` / `mysql` 等），并说明要看什么、预期输出是什么，由用户执行后回传结果，再据此判断。
- 不要假设能自行访问线上环境；也不要靠猜测给出结论，先要证据再改代码。尽量用一条命令拿到结论；涉及密钥的命令不要回显敏感值。
