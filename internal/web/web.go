package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ssh_forward/internal/registry"
	"ssh_forward/internal/store"
)

const listPageSize = 10

type pagerInfo struct {
	Show       bool
	Page       int
	TotalPages int
	Total      int
	PrevURL    string
	NextURL    string
}

func Serve(addr, webUser, webPass string, st *store.Store, reg *registry.Registry) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		switch r.URL.Path {
		case "/":
			pageHome(w, r, st, reg)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/active", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(reg.List())
	})
	mux.HandleFunc("/user/create", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		u := store.User{
			Username:      r.FormValue("username"),
			PasswordPlain: r.FormValue("password"),
			ForwardPorts:  parsePortField(r.FormValue("forward_ports")),
		}
		if strings.TrimSpace(u.Username) == "" || u.PasswordPlain == "" {
			http.Redirect(w, r, "/?err=missing", http.StatusSeeOther)
			return
		}
		if err := st.CreateUser(u); err != nil {
			http.Redirect(w, r, "/?err=create", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("/user/update", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Redirect(w, r, "/?err=id", http.StatusSeeOther)
			return
		}
		u := store.User{
			ID:           id,
			ForwardPorts: parsePortField(r.FormValue("forward_ports")),
		}
		if p := r.FormValue("password"); p != "" {
			u.PasswordPlain = p
		}
		if err := st.UpdateUser(u); err != nil {
			http.Redirect(w, r, "/?err=update", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("/user/delete", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(r.FormValue("id"), 10, 64)
		if err != nil {
			http.Redirect(w, r, "/?err=id", http.StatusSeeOther)
			return
		}
		_ = st.DeleteUser(id)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("/session/disconnect", func(w http.ResponseWriter, r *http.Request) {
		if !checkAuth(w, r, webUser, webPass) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(strings.TrimSpace(r.FormValue("conn_id")), 10, 64)
		if err != nil || id < 1 {
			http.Redirect(w, r, "/?err=badconn", http.StatusSeeOther)
			return
		}
		if !reg.DisconnectSession(id) {
			http.Redirect(w, r, "/?err=disconnect_gone", http.StatusSeeOther)
			return
		}
		redir := strings.TrimSpace(r.FormValue("redir"))
		loc := "/?ok=disconnect"
		if redir != "" {
			loc = "/?ok=disconnect&"+redir
		}
		http.Redirect(w, r, loc, http.StatusSeeOther)
	})
	return http.ListenAndServe(addr, mux)
}

func checkAuth(w http.ResponseWriter, r *http.Request, user, pass string) bool {
	u, p, ok := r.BasicAuth()
	if !ok || u != user || p != pass {
		w.Header().Set("WWW-Authenticate", `Basic realm="ssh_forward admin"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

func parsePortField(s string) []int {
	parts := strings.Split(s, ",")
	var out []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func pageHome(w http.ResponseWriter, r *http.Request, st *store.Store, reg *registry.Registry) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	users, err := st.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := r.URL.Query()
	upage := parsePage(q.Get("upage"), 1)
	apage := parsePage(q.Get("apage"), 1)

	userSlice, uPages, uTotal := paginateUsers(users, upage)
	if upage > uPages && uPages > 0 {
		upage = uPages
		userSlice, uPages, uTotal = paginateUsers(users, upage)
	}

	allActive := reg.List()
	activeSlice, aPages, aTotal := paginateActive(allActive, apage)
	if apage > aPages && aPages > 0 {
		apage = aPages
		activeSlice, aPages, aTotal = paginateActive(allActive, apage)
	}

	data := struct {
		Users       []store.User
		Active      []registry.RemoteForward
		Err         string
		OK          string
		RedirQuery  string
		PageSize    int
		UserPager   pagerInfo
		ActivePager pagerInfo
	}{
		Users:       userSlice,
		Active:      activeSlice,
		Err:         q.Get("err"),
		OK:          q.Get("ok"),
		RedirQuery:  r.URL.RawQuery,
		PageSize:    listPageSize,
		UserPager:   buildPager(r.URL.Query(), "upage", upage, uPages, uTotal),
		ActivePager: buildPager(r.URL.Query(), "apage", apage, aPages, aTotal),
	}
	if err := homeTpl.Execute(w, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parsePage(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func paginateUsers(users []store.User, page int) (slice []store.User, totalPages, total int) {
	total = len(users)
	if total == 0 {
		return nil, 1, 0
	}
	totalPages = (total + listPageSize - 1) / listPageSize
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * listPageSize
	end := start + listPageSize
	if end > total {
		end = total
	}
	return users[start:end], totalPages, total
}

func paginateActive(rows []registry.RemoteForward, page int) (slice []registry.RemoteForward, totalPages, total int) {
	total = len(rows)
	if total == 0 {
		return nil, 1, 0
	}
	totalPages = (total + listPageSize - 1) / listPageSize
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * listPageSize
	end := start + listPageSize
	if end > total {
		end = total
	}
	return rows[start:end], totalPages, total
}

func buildPager(q url.Values, pageKey string, page, totalPages, total int) pagerInfo {
	if total <= listPageSize {
		return pagerInfo{Show: false, Page: page, TotalPages: totalPages, Total: total}
	}
	pi := pagerInfo{
		Show:       true,
		Page:       page,
		TotalPages: totalPages,
		Total:      total,
	}
	if page > 1 {
		pi.PrevURL = "/?" + withPage(q, pageKey, page-1)
	}
	if page < totalPages {
		pi.NextURL = "/?" + withPage(q, pageKey, page+1)
	}
	return pi
}

func withPage(q url.Values, pageKey string, pageNum int) string {
	cp := cloneQuery(q)
	cp.Set(pageKey, strconv.Itoa(pageNum))
	return cp.Encode()
}

func cloneQuery(q url.Values) url.Values {
	cp := make(url.Values)
	for k, vs := range q {
		cp[k] = append([]string(nil), vs...)
	}
	return cp
}

func formatBytes(n int64) string {
	if n < 0 {
		n = 0
	}
	const u = 1024
	if n < u {
		return fmt.Sprintf("%d B", n)
	}
	f := float64(n)
	switch {
	case n < u*u:
		return fmt.Sprintf("%.2f KiB", f/u)
	case n < u*u*u:
		return fmt.Sprintf("%.2f MiB", f/(u*u))
	case n < u*u*u*u:
		return fmt.Sprintf("%.2f GiB", f/(u*u*u))
	default:
		return fmt.Sprintf("%.2f TiB", f/(u*u*u*u))
	}
}

func formatBps(n int64) string {
	if n <= 0 {
		return "0 B/s"
	}
	return formatBytes(n) + "/s"
}

var homeTpl = template.Must(template.New("home").Funcs(template.FuncMap{
	"fmtBytes": formatBytes,
	"fmtBps":   formatBps,
}).Parse(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>SSH 转发管理</title>
	<style>
		body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
		h1 { font-size: 1.4rem; }
		h2 { font-size: 1.1rem; margin-top: 2rem; }
		table { border-collapse: collapse; width: 100%; font-size: 0.9rem; }
		th, td { border: 1px solid #ccc; padding: 0.5rem 0.6rem; text-align: left; }
		th { background: #f4f4f4; }
		.err { color: #b00020; margin: 1rem 0; }
		.warn { color: #856404; background: #fff3cd; padding: 0.5rem 0.75rem; border-radius: 4px; }
		.okmsg { color: #155724; background: #d4edda; padding: 0.5rem 0.75rem; border-radius: 4px; margin: 1rem 0; }
		input[type=text], input[type=password], textarea { width: 100%; box-sizing: border-box; padding: 0.4rem; }
		textarea { min-height: 3rem; font-family: inherit; }
		button { padding: 0.35rem 0.75rem; cursor: pointer; }
		.mono { font-family: ui-monospace, monospace; font-size: 0.85rem; }
		.hint { font-size: 0.85rem; color: #555; margin-top: 0.25rem; }
		.pager { font-size: 0.85rem; margin: 0.5rem 0 0; color: #444; }
		.pager a { margin: 0 0.25rem; }
		.pager .disabled { color: #999; }
	</style>
</head>
<body>
	<h1>SSH 端口转发服务</h1>
	<p class="hint">本服务<strong>仅支持</strong>远程转发 <code>ssh -R</code>（不支持 <code>-L</code>）。下方「允许的远程转发端口」即 <code>-R 端口:目标:目标端口</code> 中服务端监听端口，须与此处登记一致。</p>
	<p class="hint warn">列表中的密码以<strong>明文</strong>写入数据库便于查看；请勿对公网暴露管理页，勿提交 <code>app.db</code>。</p>
	{{if .Err}}<p class="err">操作未完成（错误代码：{{.Err}}）</p>{{end}}
	{{if eq .OK "disconnect"}}<p class="okmsg">已断开该客户端的 SSH 连接（其全部远程转发已结束）。</p>{{end}}

	<h2>当前远程转发监听（-R）</h2>
	<p class="hint">「入/出」相对<strong>本机监听端口</strong>上的流量：入=外部连接写入，出=写回外部。当前网速为最近 1 秒平均；本会话流量为当前 SSH 连接上该会话累计。</p>
	<table>
		<tr><th>SSH 用户</th><th>监听端口</th><th>客户端地址</th><th>会话 ID</th><th>开始时间</th><th>当前网速</th><th>本会话流量</th><th>操作</th></tr>
		{{range .Active}}
		<tr>
			<td>{{.Username}}</td>
			<td class="mono">{{.Port}}</td>
			<td class="mono">{{.ClientAddr}}</td>
			<td class="mono">{{.ConnID}}</td>
			<td class="mono">{{.Since.Format "2006-01-02 15:04:05"}}</td>
			<td class="mono" title="入站=外部→SSH，出站=SSH→外部">入 {{fmtBps .RateRxBps}}<br>出 {{fmtBps .RateTxBps}}</td>
			<td class="mono">入 {{fmtBytes .BytesRx}}<br>出 {{fmtBytes .BytesTx}}</td>
			<td>
				<form method="post" action="/session/disconnect" style="margin:0" onsubmit="return confirm('断开此 SSH 客户端？同一客户端上所有 -R 将一并结束。');">
					<input type="hidden" name="conn_id" value="{{.ConnID}}">
					<input type="hidden" name="redir" value="{{$.RedirQuery}}">
					<button type="submit">断开</button>
				</form>
			</td>
		</tr>
		{{else}}
		<tr><td colspan="8">暂无活跃监听</td></tr>
		{{end}}
	</table>
	{{if .ActivePager.Show}}
	<p class="pager">共 {{.ActivePager.Total}} 条；每页 {{.PageSize}} 条。
		{{if .ActivePager.PrevURL}}<a href="{{.ActivePager.PrevURL}}">上一页</a>{{else}}<span class="disabled">上一页</span>{{end}}
		第 {{.ActivePager.Page}} / {{.ActivePager.TotalPages}} 页
		{{if .ActivePager.NextURL}}<a href="{{.ActivePager.NextURL}}">下一页</a>{{else}}<span class="disabled">下一页</span>{{end}}
	</p>
	{{end}}

	<h2>SSH 账号与允许端口</h2>
	<table>
		<tr><th>用户</th><th>密码</th><th>历史流量汇总</th><th>允许的远程转发端口</th><th>更新</th><th>删除</th></tr>
		{{range .Users}}
		<tr>
			<td class="mono">{{.Username}}</td>
			<td class="mono">{{if .StoredPassword}}{{.StoredPassword}}{{else}}<em title="在该功能上线前创建的用户，或从未通过本页保存过密码">—</em>{{end}}</td>
			<td class="mono" title="历次 SSH 会话结束后累计（按转发套接字统计）">入 {{fmtBytes .TrafficRxTotal}}<br>出 {{fmtBytes .TrafficTxTotal}}</td>
			<td class="mono">{{range $i, $p := .ForwardPorts}}{{if $i}}, {{end}}{{$p}}{{else}}<em>无（禁止 -R）</em>{{end}}</td>
			<td>
				<details>
					<summary>编辑</summary>
					<form method="post" action="/user/update" style="margin-top:.5rem;">
						<input type="hidden" name="id" value="{{.ID}}">
						<label>新密码（留空则不修改）</label>
						<input type="password" name="password" autocomplete="new-password">
						<label>允许的远程转发端口（逗号分隔）</label>
						<textarea name="forward_ports">{{range $i, $p := .ForwardPorts}}{{if $i}},{{end}}{{$p}}{{end}}</textarea>
						<button type="submit">保存</button>
					</form>
				</details>
			</td>
			<td>
				<form method="post" action="/user/delete" onsubmit="return confirm('删除用户 {{.Username}}？');">
					<input type="hidden" name="id" value="{{.ID}}">
					<button type="submit">删除</button>
				</form>
			</td>
		</tr>
		{{else}}
		<tr><td colspan="6">尚无用户，请在下方添加</td></tr>
		{{end}}
	</table>
	{{if .UserPager.Show}}
	<p class="pager">共 {{.UserPager.Total}} 个用户；每页 {{.PageSize}} 条。
		{{if .UserPager.PrevURL}}<a href="{{.UserPager.PrevURL}}">上一页</a>{{else}}<span class="disabled">上一页</span>{{end}}
		第 {{.UserPager.Page}} / {{.UserPager.TotalPages}} 页
		{{if .UserPager.NextURL}}<a href="{{.UserPager.NextURL}}">下一页</a>{{else}}<span class="disabled">下一页</span>{{end}}
	</p>
	{{end}}

	<h2>添加用户</h2>
	<form method="post" action="/user/create">
		<label>用户名</label>
		<input type="text" name="username" required autocomplete="username">
		<label>密码</label>
		<input type="password" name="password" required autocomplete="new-password">
		<label>允许的远程转发端口（逗号分隔，例如 <span class="mono">8080,8443</span>）</label>
		<textarea name="forward_ports" placeholder="8080,9000"></textarea>
		<p class="hint">至少需要登记一个端口，该用户才能使用 <code>ssh -R</code>。</p>
		<button type="submit">创建</button>
	</form>
</body>
</html>
`))
