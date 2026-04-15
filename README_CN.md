# ssh_forward

English: [README.md](README.md)

基于 **Go** 的轻量 **SSH 服务端**，支持 **远程 TCP 转发 (`-R`)**。**本地转发 `-L` / `direct-tcpip` 默认关闭**，避免服务端代拨任意目标；需要时可通过启动参数 **`-allow-local-forward`** 显式开启。带 **Web 管理界面**：维护 SSH 登录账号/密码，以及每个客户端允许使用的 **远程转发端口**，并在页面上查看当前活跃的 `-R` 监听。

## 功能概览

- **SSH**：密码认证（bcrypt 存储）；主机密钥首次运行自动生成（Ed25519，保存在数据目录）。
- **远程转发 (`-R`)**：仅在 Web 中为该用户登记的端口上监听；未配置端口则拒绝 `tcpip-forward`。
- **本地转发 (`-L`)**：**默认关闭**（拒绝 `direct-tcpip`）；使用 **`-allow-local-forward`** 可开启（服务端会按客户端请求代拨 TCP）。
- **Web**：HTTP Basic 认证；用户增删改、维护「允许的远程转发端口」列表；展示当前 `-R` 监听及 API `GET /api/active`（JSON）。

## 环境要求

- Go **1.18+**（推荐与 `go.mod` 一致）

## 构建与运行

```bash
go build -o ssh_forward .
./ssh_forward -web-pass='你的管理后台密码'
```

`-web-pass` 为 **必填**，用于保护 Web 管理页。

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-data` | `./data` | 数据目录：SQLite (`app.db`)、主机私钥 (`ssh_host_ed25519`) |
| `-ssh` | `:2222` | SSH 监听地址 |
| `-http` | `127.0.0.1:8080` | Web 管理监听地址（默认仅本机） |
| `-web-user` | `admin` | Web Basic 用户名 |
| `-web-pass` | （无） | Web Basic 密码，**必填** |
| `-allow-local-forward` | `false` | 开启 SSH 本地转发（`-L` / `direct-tcpip`）；不可信客户端可连时**风险高** |

日志中会打印 Web 访问地址与 `-web-user`。

## Web 管理说明

1. 浏览器访问 `-http` 配置的地址，使用 `-web-user` / `-web-pass` 登录。
2. **添加用户**：设置 SSH 用户名、密码，以及「允许的远程转发端口」（逗号分隔，如 `8080,8443`）。
3. **编辑用户**：可改端口列表；填写新密码则更新密码，留空则不改密码。
4. 首页表格可查看 **当前 `-R` 监听**（用户、端口、SSH 客户端地址、会话编号、登记时间等），并可 **断开** 某条会话对应的 SSH 客户端（会关闭该连接上的全部转发）。

## SSH 客户端示例

远程转发：将「SSH 客户端一侧」的 `127.0.0.1:3000` 暴露到「SSH 服务器」的 `8080`（`8080` 须已在 Web 中为该用户授权）：

```bash
ssh -N -p 2222 -R 8080:127.0.0.1:3000 用户名@服务器地址
```

默认情况下 `ssh -L` 会被拒绝。若服务端以 **`-allow-local-forward`** 启动，则支持 `-L`（服务端会向客户端指定的主机/端口发起连接，请充分评估风险）。

示例（在客户端监听 `8080`，经服务端转发到服务端视角下的 `127.0.0.1:3000`）：

```bash
ssh -N -p 2222 -L 8080:127.0.0.1:3000 用户名@服务器地址
```

首次连接若需自动接受主机密钥，可酌情使用 `StrictHostKeyChecking=accept-new`（请自行评估安全策略）。

## Windows 客户端：断线自动重连（.bat）

目录 [`scripts/windows/`](scripts/windows/) 提供批处理，用于在网络抖动导致 SSH 退出后 **定时重新建立** 端口转发。

1. 将 `ssh_forward_reconnect_config.example.cmd` 复制为同目录下的 `ssh_forward_reconnect_config.cmd`（不要提交到版本库）。
2. 编辑配置：服务器地址、端口、`SSH_USER`、**`MODE=R`**（仅远程转发；连接本服务勿用 `MODE=L`）及端口与目标。
3. **密码登录**：安装 [PuTTY](https://www.chiark.greenend.org.uk/~sgtatham/putty/)，设置 `SSH_PASS` 与 `PLINK` 路径；首次无人值守建议配置 `SSH_HOSTKEY`（见示例内说明）。
4. **密钥登录**：删除 `SSH_PASS`，设置 `SSH_KEY` 指向私钥；使用系统自带 `ssh`（OpenSSH），带 `ServerAliveInterval` 便于更快感知断线。

双击或在终端运行 `ssh_forward_reconnect.bat`；按 Ctrl+C 停止。`RECONNECT_SEC` 控制重试间隔。

## 项目结构

```
internal/
  sshd/       SSH 服务、端口转发逻辑
  store/      SQLite 与用户信息
  registry/   活跃 -R 监听登记（供 Web 展示）
  web/        HTTP 管理界面与 /api/active
scripts/windows/   Windows 自动重连 bat + 配置示例
main.go       入口
```

## 安全提示

- 管理后台默认只监听本机；若需远程访问，建议放在反向代理后并启用 **HTTPS**，或限制来源 IP。
- 生产环境应使用防火墙限制 **SSH 端口** 访问来源。
- 绑定 **1024 以下** 特权端口通常需要更高权限；优先使用高端口。
- **不要**在仓库或日志中泄露 `-web-pass`、用户密码或主机私钥。
- Web 列表展示依赖在 SQLite 中 **明文保存** 的 SSH 密码副本（与 bcrypt 哈希并存）；请严格保护 `app.db` 与管理入口。
- 开启 **`-allow-local-forward`** 后，已认证客户端可让服务端连接其指定的目标，勿对不可信用户开放，除非另有网络或策略约束。
