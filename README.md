<p align="center"><img src="docs/logo.svg" width="120"></p>

# TickFlow

开源的多租户、多场景工单平台。同一套平台上，不同租户、不同类型的工单可以拥有不同的表单、流程、SLA、分派策略与界面展示，而不必为每个场景单独搭建一套系统。

工单类型被实现为**模板包**：表单 schema、状态机流程、SLA 策略、分派策略与视图配置五件事打包在一起。租户启用哪些模板包，就获得哪些场景；提交门户与处理工作台均由 schema 驱动渲染，不为任何场景手写页面。

## 场景

| 场景 | 提交人 | 处理人 | 特点 |
|---|---|---|---|
| 内部 IT / 运维支持 | 内部员工 | 技术团队 | 报障、权限申请 |
| 客户售后 / 客服 | 外部用户 | 客服 / 售后团队 | 满意度、SLA |
| 设备维修 / 外勤派单 | 设备使用方 | 维修工程师 | 现场照片、上门信息 |
| 流程审批 | 员工 | 审批节点处理人 | 顺序 / 会签审批 |

以上四类场景随仓库内置的种子模板包开箱可用，也可以在管理台中自定义新的工单类型。

## 快速开始

一条命令启动（需要可访问的 PostgreSQL）：

```bash
docker run -d --name tickflow \
  -p 8080:8080 \
  -v tickflow-data:/data \
  -e TICKFLOW_DB_DSN='postgres://tickflow:tickflow@your-postgres:5432/tickflow?sslmode=disable' \
  -e TICKFLOW_JWT_SECRET='change-me-to-a-long-random-string' \
  ghcr.io/linuxsuren/tick-flow:latest
```

浏览器打开 `http://localhost:8080`：

1. 首次访问会进入初始化引导页，创建工作区与管理员账号；初始化只允许执行一次；
2. 登录后自动获得四个内置工单类型，即可提交与处理工单；
3. 在「管理」中邀请成员、定制品牌色，或用 JSON 编辑器调整工单类型。

其他安装方式：

```bash
# 从源码安装（需要 Go 1.26+，前端产物已嵌入）
go install github.com/LinuxSuRen/tick-flow/cmd/server@latest

# 或从 GitHub Releases 下载对应平台的压缩包
# https://github.com/LinuxSuRen/tick-flow/releases
```

开发模式：

```bash
go run ./cmd/server          # 后端，默认监听 :8080
pnpm --dir web dev           # 前端开发服务器，/api 自动代理到后端
```

## 配置

配置通过环境变量提供，也可用 YAML 文件覆盖默认值（`TICKFLOW_CONFIG` 指定路径，环境变量优先）。

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `TICKFLOW_ADDR` | `:8080` | HTTP 监听地址 |
| `TICKFLOW_DB_DSN` | `postgres://tickflow:tickflow@localhost:5432/tickflow?sslmode=disable` | PostgreSQL 连接串 |
| `TICKFLOW_JWT_SECRET` | 随机生成 | JWT 签名密钥；未设置时每次启动随机生成，重启后登录态失效，生产环境务必显式配置 |
| `TICKFLOW_UPLOAD_DIR` | `data/uploads` | 附件存储目录 |
| `TICKFLOW_UPLOAD_MAX_BYTES` | `20971520` | 单个附件大小上限，默认 20MB |
| `TICKFLOW_UPLOAD_EXT_WHITELIST` | `.png,.jpg,.jpeg,.webp,.gif,.pdf,.zip,.txt` | 允许上传的扩展名 |
| `TICKFLOW_SMTP_HOST` | 空 | SMTP 服务器；留空时邮件通知停用，邀请链接输出到服务端日志 |
| `TICKFLOW_SMTP_PORT` | `587` | SMTP 端口 |
| `TICKFLOW_SMTP_USERNAME` / `TICKFLOW_SMTP_PASSWORD` | 空 | SMTP 认证 |
| `TICKFLOW_SMTP_FROM` | 同用户名 | 发件人地址 |
| `TICKFLOW_LOG_LEVEL` | `info` | 日志级别：debug / info / warn / error |

数据库表结构由 [golang-migrate](https://github.com/golang-migrate/migrate) 迁移管理，服务启动时自动执行。API 契约见 [docs/openapi.yaml](docs/openapi.yaml)。

## Roadmap

- **P0 平台底座**（当前）：租户与成员、工单核心、模板包、通知、门户与工作台；
- **P1 专业能力**：流程配置器、SLA 计时与升级、自动分派、报表、开放 API 与 webhook；
- **P2 场景扩展**：审批链引擎、外勤移动端、邮件转工单、模板包导入导出。

完整路线图与任务拆解见 [#1](https://github.com/LinuxSuRen/tick-flow/issues/1)。

## 如何贡献

- 从 [issue 列表](https://github.com/LinuxSuRen/tick-flow/issues) 认领任务，标注 `good first issue` 的适合上手；认领后先在 issue 下留言说明计划；
- 开发约定：不直推 `master`，feature 分支 + PR 合入；提交前自测通过（后端 `go build ./... && go test ./...`，前端 `pnpm --dir web run build`）；
- PR 描述关联对应 issue（如 `close #13`），一个 PR 只包含一类修改；
- 集成测试依赖 Docker（testcontainers 自动拉起临时 PostgreSQL）：`go test ./test/integration/`。

## License

[Apache-2.0](LICENSE)
