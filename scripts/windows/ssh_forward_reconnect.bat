@echo off
setlocal EnableExtensions EnableDelayedExpansion
cd /d "%~dp0"

set "CONFIG=%~dp0ssh_forward_reconnect_config.cmd"
if not exist "%CONFIG%" (
    echo [ERROR] Config file not found:
    echo   %CONFIG%
    echo Copy ssh_forward_reconnect_config.example.cmd to
    echo   ssh_forward_reconnect_config.cmd and fill in the values.
    exit /b 1
)

call "%CONFIG%"

if not defined SSH_HOST (
    echo [ERROR] Set SSH_HOST in ssh_forward_reconnect_config.cmd.
    exit /b 1
)
if not defined SSH_USER (
    echo [ERROR] Set SSH_USER in ssh_forward_reconnect_config.cmd.
    exit /b 1
)

if not defined SSH_PORT set "SSH_PORT=2222"
if not defined RECONNECT_SEC set "RECONNECT_SEC=5"
if not defined MODE set "MODE=R"
if not defined SSH_STRICT_HOST_KEY set "SSH_STRICT_HOST_KEY=accept-new"

if /i "!MODE!"=="R" (
    if not defined SERVER_BIND_PORT (
        echo [ERROR] MODE=R requires SERVER_BIND_PORT ^(server listen port^).
        exit /b 1
    )
    if not defined TARGET_HOST set "TARGET_HOST=127.0.0.1"
    if not defined TARGET_PORT (
        echo [ERROR] MODE=R requires TARGET_PORT ^(local target port^).
        exit /b 1
    )
) else if /i "!MODE!"=="L" (
    if not defined LOCAL_BIND_PORT (
        echo [ERROR] MODE=L requires LOCAL_BIND_PORT ^(local listen port^).
        exit /b 1
    )
    if not defined TARGET_HOST (
        echo [ERROR] MODE=L requires TARGET_HOST.
        exit /b 1
    )
    if not defined TARGET_PORT (
        echo [ERROR] MODE=L requires TARGET_PORT.
        exit /b 1
    )
) else (
    echo [ERROR] MODE must be R or L; current value is !MODE!
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
        echo [ERROR] SSH_PASS is set; PuTTY plink.exe is required.
        echo Install PuTTY or set PLINK= to the full path of plink.exe.
        echo Download: https://www.chiark.greenend.org.uk/~sgtatham/putty/
        exit /b 1
    )
)

echo.
echo ========== ssh_forward auto-reconnect ==========
echo Host: !SSH_USER!@!SSH_HOST!:!SSH_PORT!
echo Mode: !MODE!
if /i "!MODE!"=="R" (
    echo Forward: -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT!
) else (
    echo Forward: -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT!
)
if "!USE_PLINK!"=="1" (
    echo Auth: plink + password ^(do not store secrets on shared machines^)
) else (
    echo Auth: OpenSSH ssh + private key BatchMode
)
echo Reconnect interval: !RECONNECT_SEC! s
echo Press Ctrl+C to stop this script.
echo ================================================
echo.

:loop
echo [!date! !time:~0,8!] Establishing SSH forward...
if "!USE_PLINK!"=="1" (
    call :run_plink
) else (
    call :run_openssh
)
set "EXITCODE=!ERRORLEVEL!"
echo [!date! !time:~0,8!] Session ended ^(exit code !EXITCODE!^), retrying in !RECONNECT_SEC! s...
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
    echo [ERROR] When SSH_PASS is not set, set SSH_KEY to your private key file path.
    exit /b 2
)
if not exist "!SSH_KEY!" (
    echo [ERROR] Private key not found: !SSH_KEY!
    exit /b 2
)
if /i "!MODE!"=="R" (
    ssh -o BatchMode=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=!SSH_STRICT_HOST_KEY! -i "!SSH_KEY!" -p !SSH_PORT! -N -R !SERVER_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
) else (
    ssh -o BatchMode=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -o StrictHostKeyChecking=!SSH_STRICT_HOST_KEY! -i "!SSH_KEY!" -p !SSH_PORT! -N -L !LOCAL_BIND_PORT!:!TARGET_HOST!:!TARGET_PORT! !SSH_USER!@!SSH_HOST!
)
exit /b %ERRORLEVEL%
