@echo off
REM 复制本文件为 ssh_forward_reconnect_config.cmd 再修改。
REM 不要提交 ssh_forward_reconnect_config.cmd（内含密码）。

REM ========== SSH 登录 ==========
set "SSH_HOST=你的服务器域名或IP"
set "SSH_PORT=2222"
set "SSH_USER=ssh用户名"

REM 使用密码自动重连：安装 PuTTY，并设置 SSH_PASS（明文，注意仅本机权限）。
REM 若改用密钥，删除下行 SSH_PASS，并设置 SSH_KEY。
REM 注意：脚本启用延迟变量展开，极个别符号可能导致密码异常，可改用 SSH_KEY。
set "SSH_PASS=你的SSH密码"

REM 私钥路径（留空 SSH_PASS 时生效）。需配合 ssh-copy-id 或已在服务端登记公钥。
REM set "SSH_KEY=%USERPROFILE%\.ssh\id_ed25519"

REM plink.exe 路径；若在 PATH 中可直接写 plink
set "PLINK=C:\Program Files\PuTTY\plink.exe"

REM 无人值守时建议填写服务端主机密钥，避免 plink -batch 因指纹变更失败。
REM 获取指纹：首次手动 plink 或 ssh 连接后，在 PuTTY 缓存中查看，或查服务器文档。
REM set "SSH_HOSTKEY=ssh-ed25519:255:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

REM ========== 转发模式 ==========
REM MODE=R  远程转发（本 ssh_forward 服务仅支持此模式）
REM MODE=L  一般 OpenSSH 本地转发；连接本服务时请固定 MODE=R，否则会遭服务端拒绝

set "MODE=R"

REM ---- MODE=R（对应 ssh -R）----
set "SERVER_BIND_PORT=8080"
set "TARGET_HOST=127.0.0.1"
set "TARGET_PORT=3000"

REM ---- MODE=L（对应 ssh -L）----
REM set "MODE=L"
REM set "LOCAL_BIND_PORT=9000"
REM set "TARGET_HOST=内网或服务器能访问的主机"
REM set "TARGET_PORT=80"

REM ========== 其它 ==========
REM 断线后等待秒数再重连
set "RECONNECT_SEC=5"

REM OpenSSH 可选：接受新主机密钥（首次连接方便，生产请改为 yes 并预置 known_hosts）
set "SSH_STRICT_HOST_KEY=accept-new"
