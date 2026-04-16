# ssh_forward

中文文档：[README_CN.md](README_CN.md)

A lightweight **SSH server** in **Go** that supports **remote TCP forwarding (`-R`)**. **Client-side TCP forwarding (`-L`, `-D` / `direct-tcpip`) is on by default** via **`-allow-dynamic-forward`** (default `true`); the server dials targets the client requests. Disable with **`-allow-dynamic-forward=false`** and leave **`-allow-local-forward`** off if you do not want that behavior. A **web admin UI** manages SSH accounts/passwords, **allowed remote forward ports** per user, and shows active `-R` listeners.

## Features

- **SSH**: Password authentication (bcrypt); host key generated on first run (Ed25519, under the data directory).
- **Remote forwarding (`-R`)**: Listens only on ports you allow in the web UI; `tcpip-forward` is denied for unlisted ports.
- **Local (`-L`) and dynamic (`-D`) forwarding**: **On by default** (`-allow-dynamic-forward` defaults to `true`; same `direct-tcpip` path for **`-L`** and SOCKS **`-D`**). Set **`-allow-dynamic-forward=false`** and do not set **`-allow-local-forward`** to reject `direct-tcpip`.
- **Web**: HTTP Basic auth; CRUD users and allowed ports; active `-R` list; `GET /api/active` (JSON).

## Requirements

- Go **1.18+** (see `go.mod`)

## Build and run

```bash
go build -o ssh_forward .
./ssh_forward -web-pass='your-admin-password'
```

`-web-pass` is **required** to protect the web UI.

## Command-line flags

| Flag | Default | Description |
|------|---------|-------------|
| `-data` | `./data` | Data directory: SQLite (`app.db`), host key (`ssh_host_ed25519`) |
| `-ssh` | `:2222` | SSH listen address |
| `-http` | `127.0.0.1:8080` | Web admin listen address (localhost by default) |
| `-web-user` | `admin` | Web Basic username |
| `-web-pass` | (none) | Web Basic password (**required**) |
| `-allow-local-forward` | `false` | Enable SSH static local forwarding (`-L` / `direct-tcpip`); **dangerous** if untrusted clients can connect |
| `-allow-dynamic-forward` | `true` | Enable SSH dynamic SOCKS forwarding (`-D` / same `direct-tcpip` path as `-L`); use `false` to disable unless `-allow-local-forward` is set; **dangerous** if untrusted clients can connect |

Logs print the web URL and `-web-user`.

## Web admin

1. Open the `-http` URL in a browser; sign in with `-web-user` / `-web-pass`.
2. **Create user**: SSH username, password, and comma-separated **allowed remote forward ports** (e.g. `8080,8443`).
3. **Edit user**: Change ports; set a new password or leave blank to keep the current one.
4. The dashboard lists **active `-R` listeners** (user, port, client address, session id, start time, etc.) and lets you **disconnect** a client (closes that SSH session and all its forwards).

## SSH client example

Expose `127.0.0.1:3000` on the client to port `8080` on the server (port `8080` must be allowed for that user in the web UI):

```bash
ssh -N -p 2222 -R 8080:127.0.0.1:3000 user@server
```

By default **`-allow-dynamic-forward`** is `true`, so **`-L`** and **`-D`** work without extra flags. Turn off with **`-allow-dynamic-forward=false`** (and omit **`-allow-local-forward`**) if you want `direct-tcpip` disabled.

Static local forward — listen on the client at `8080`, forward via the server to `127.0.0.1:3000` **as seen from the server**:

```bash
ssh -N -p 2222 -L 8080:127.0.0.1:3000 user@server
```

Dynamic forward — SOCKS proxy on the client at `1080` (traffic exits **from the server** toward the target host):

```bash
ssh -N -p 2222 -D 1080 user@server
```

With **`-allow-dynamic-forward=false`** and **`-allow-local-forward`** not set, `direct-tcpip` is rejected.

For first-time host key acceptance you may use `StrictHostKeyChecking=accept-new` (evaluate risk for your environment).

## Windows: auto-reconnect (.bat)

See [`scripts/windows/`](scripts/windows/) for a batch script that reconnects after network drops.

1. Copy `ssh_forward_reconnect_config.example.cmd` to `ssh_forward_reconnect_config.cmd` (do not commit secrets).
2. Edit host, port, `SSH_USER`, and **`MODE=R`** only (use `MODE=L` only against other SSH servers, not this one).
3. **Password**: Install [PuTTY](https://www.chiark.greenend.org.uk/~sgtatham/putty/), set `SSH_PASS` and `PLINK`; for unattended runs set `SSH_HOSTKEY` (see the example).
4. **Key auth**: Clear `SSH_PASS`, set `SSH_KEY` to your private key; OpenSSH `ssh` is used with `ServerAliveInterval`/`ServerAliveCountMax`.

Run `ssh_forward_reconnect.bat`; Ctrl+C to stop. `RECONNECT_SEC` sets the retry delay.

## Project layout

```
internal/
  sshd/       SSH server and forwarding
  store/      SQLite and users
  registry/   Active -R entries for the web UI
  web/        HTTP admin and /api/active
scripts/windows/   Windows reconnect script + example config
main.go       Entry point
```

## Security

- Bind the admin UI to localhost by default; for remote access use a reverse proxy with **HTTPS** or IP restrictions.
- Restrict **SSH** with a firewall in production.
- Ports **below 1024** often need elevated privileges; prefer high ports.
- Do not commit **`-web-pass`**, user passwords, or the host private key.
- The web UI can show **plaintext** SSH passwords stored in SQLite alongside bcrypt hashes; protect **`app.db`** and the admin endpoint.
- **`-allow-dynamic-forward`** defaults to **`true`**, so **`-L` / `-D`** behave like a generic TCP/SOCKS proxy unless you disable it; **`-allow-local-forward`** adds the same capability if dynamic is off. Do not expose SSH to untrusted users without additional controls.

## License

Add a license appropriate for your use case.
