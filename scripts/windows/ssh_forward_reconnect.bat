@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

set "CONFIG=%~dp0ssh_forward_reconnect_config.cmd"
if not exist "%CONFIG%" (
    echo [错误] 未找到配置文件：
    echo   %CONFIG%
    echo 请先复制 ssh_forward_reconnect_config.example.cmd 为
    echo   ssh_forward_reconnect_config.cmd 并按说明填写。
    exit /b 1
)

call "%CONFIG%"

if not defined SSH_HOST (
    echo [错误] 请在 ssh_forward_reconnect_config.cmd 中设置 SSH_HOST。
    exit /b 1
)
if not defined SSH_USER (
    echo [错误] 请在 ssh_forward_reconnect_config.cmd 中设置 SSH_USER。
    exit /b 1
)

if not defined SSH_PORT set "SSH_PORT=2222"
if not defined RECONNECT_SEC set "RECONNECT_SEC=5"
if not defined MODE set "MODE=R"
if not defined SSH_STRICT_HOST_KEY set "SSH_STRICT_HOST_KEY=accept-new"

if /i "!MODE!"=="R" (
    if not defined SERVER_BIND_PORT (
        echo [错误] MODE=R 时需要 SERVER_BIND_PORT（服务端监听端口）。
        exit /b 1
    )
    if not defined TARGET_HOST set "TARGET_HOST=127.0.0.1"
    if not defined TARGET_PORT (
        echo [错误] MODE=R 时需要 TARGET_PORT（本机目标端口）。
        exit /b 1
    )
) else if /i "!MODE!"=="L" (
    if not defined LOCAL_BIND_PORT (
        echo [错误] MODE=L 时需要 LOCAL_BIND_PORT（本机监听端口）。
        exit /b 1
    )
    if not defined TARGET_HOST (
        echo [错误] MODE=L 时需要 TARGET_HOST。
        exit /b 1
    )
    if not defined TARGET_PORT (
        echo [错误] MODE=L 时需要 TARGET_PORT。
        exit /b 1
    )
) else (
    echo [错误] MODE 必须是 R 或 L，当前为 !MODE!
    exit /b 1
)

set "USE_PLINK=0"
if defined SSH_PASS set "USE_PLINK=1"

if "!USE_PLINK!"=="1" (
    if not defined PLINK set "PLINK=plink"
    set "PLINK_OK=0"
    if exist "!PLINK!" set "PLINK_OK=1"
    if "!PLINK_OK!"=="0" (
        where plink >nul 2>&1
        if not errorlevel 1 set "PLINK_OK=1"
    )
    if "!PLINK_OK!"=="0" (
        echo [错误] 已设置 SSH_PASS，需要 PuTTY 的 plink.exe。
        echo 请安装 PuTTY 或在配置里设置 PLINK= 为 plink.exe 的完整路径。
        echo 下载: https://www.chiark.greenend.org.uk/~sgtatham/putty/
        exit /b 1
    )
)

echo.
echo ========== ssh_forward 自动重连 ==========
echo 主机: !SSH_USER!@!SSH_HOST!:!SSH_PORT!
echo 模式: !MODE!
if /i "!MODE!"=="R" (
    echo 转发: -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT!
) else (
    echo 转发: -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT!
)
if "!USE_PLINK!"=="1" (
    echo 认证: plink + 密码 ^(请勿在公共电脑上保存配置^)
) else (
    echo 认证: OpenSSH ssh + 私钥 BatchMode
)
echo 重连间隔: !RECONNECT_SEC! 秒
echo 按 Ctrl+C 停止本脚本。
echo ==========================================
echo.

:loop
echo [!date! !time:~0,8!] 正在建立 SSH 转发...
if "!USE_PLINK!"=="1" (
    call :run_plink
) else (
    call :run_openssh
)
set "EXITCODE=!ERRORLEVEL!"
echo [!date! !time:~0,8!] 会话已结束 ^(退出码 !EXITCODE!^)，!RECONNECT_SEC! 秒后重试...
timeout /t !RECONNECT_SEC! /nobreak >nul
goto loop

:run_plink
set "PLINK_EXE=!PLINK!"
if not exist "!PLINK_EXE!" (
    where plink >nul 2>&1
    if not errorlevel 1 set "PLINK_EXE=plink"
)

if defined SSH_HOSTKEY (
    if /i "!MODE!"=="R" (
        "!PLINK_EXE!" -ssh -batch -P !SSH_PORT! -pw "!SSH_PASS!" -hostkey "!SSH_HOSTKEY!" -N -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
    ) else (
        "!PLINK_EXE!" -ssh -batch -P !SSH_PORT! -pw "!SSH_PASS!" -hostkey "!SSH_HOSTKEY!" -N -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
    )
) else (
    if /i "!MODE!"=="R" (
        "!PLINK_EXE!" -ssh -batch -P !SSH_PORT! -pw "!SSH_PASS!" -N -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
    ) else (
        "!PLINK_EXE!" -ssh -batch -P !SSH_PORT! -pw "!SSH_PASS!" -N -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
    )
)
exit /b %ERRORLEVEL%

:run_openssh
if not defined SSH_KEY (
    echo [错误] 未设置 SSH_PASS 时，请在配置中设置 SSH_KEY 指向私钥文件。
    exit /b 2
)
if not exist "!SSH_KEY!" (
    echo [错误] 找不到私钥: !SSH_KEY!
    exit /b 2
)
if /i "!MODE!"=="R" (
    ssh -o BatchMode=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=!SSH_STRICT_HOST_KEY! -i "!SSH_KEY!" -p !SSH_PORT! -N -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
) else (
    ssh -o BatchMode=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=!SSH_STRICT_HOST_KEY! -i "!SSH_KEY!" -p !SSH_PORT! -N -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
)
exit /b %ERRORLEVEL%
