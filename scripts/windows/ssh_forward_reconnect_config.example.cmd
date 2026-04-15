@echo off
REM Copy this file to ssh_forward_reconnect_config.cmd and edit.
REM Do not commit ssh_forward_reconnect_config.cmd (it may contain secrets).

REM ========== SSH login ==========
set "SSH_HOST=your.server.example.com"
set "SSH_PORT=2222"
set "SSH_USER=ssh_username"

REM Password auto-reconnect: install PuTTY and set SSH_PASS (plaintext; keep file permissions tight).
REM For key auth, remove SSH_PASS below and set SSH_KEY.
REM Note: delayed expansion is enabled; rare characters in passwords may break; prefer SSH_KEY if needed.
set "SSH_PASS=your_ssh_password"

REM Private key path (used when SSH_PASS is empty). Use ssh-copy-id or register the pubkey on the server.
REM set "SSH_KEY=%USERPROFILE%\.ssh\id_ed25519"

REM Path to plink.exe; if plink is on PATH, you can use plink
set "PLINK=C:\Program Files\PuTTY\plink.exe"

REM For unattended runs, set the server host key so plink -batch does not fail on fingerprint changes.
REM Get fingerprint: first manual plink/ssh session, PuTTY cache, or server docs.
REM set "SSH_HOSTKEY=ssh-ed25519:255:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

REM ========== Forward mode ==========
REM MODE=R  Remote forward (default for this ssh_forward service)
REM MODE=L  Typical OpenSSH local forward; use MODE=R with this service unless -allow-local-forward is enabled

set "MODE=R"

REM ---- MODE=R (same as ssh -R) ----
set "SERVER_BIND_PORT=8080"
set "TARGET_HOST=127.0.0.1"
set "TARGET_PORT=3000"

REM ---- MODE=L (same as ssh -L) ----
REM set "MODE=L"
REM set "LOCAL_BIND_PORT=9000"
REM set "TARGET_HOST=host reachable from the server"
REM set "TARGET_PORT=80"

REM ========== Other ==========
REM Seconds to wait after disconnect before reconnecting
set "RECONNECT_SEC=5"

REM OpenSSH optional: accept new host keys (handy for first connect; use yes + known_hosts in production)
set "SSH_STRICT_HOST_KEY=accept-new"
