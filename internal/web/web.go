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

func jsQuote(s string) template.HTML {
	b, err := json.Marshal(s)
	if err != nil {
		return template.HTML(`""`)
	}
	return template.HTML(b)
}

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
			homeRedirect(w, r, url.Values{"err": {"missing"}})
			return
		}
		if err := st.CreateUser(u); err != nil {
			homeRedirect(w, r, url.Values{"err": {"create"}})
			return
		}
		homeRedirect(w, r, nil)
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
			homeRedirect(w, r, url.Values{"err": {"id"}})
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
			homeRedirect(w, r, url.Values{"err": {"update"}})
			return
		}
		homeRedirect(w, r, nil)
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
			homeRedirect(w, r, url.Values{"err": {"id"}})
			return
		}
		_ = st.DeleteUser(id)
		homeRedirect(w, r, nil)
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
			homeRedirect(w, r, url.Values{"err": {"badconn"}})
			return
		}
		if !reg.DisconnectSession(id) {
			homeRedirect(w, r, url.Values{"err": {"disconnect_gone"}})
			return
		}
		redirectAfterDisconnect(w, r, strings.TrimSpace(r.FormValue("redir")))
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
	lang := applyLangCookie(w, r)
	L := localeFor(lang)

	users, err := st.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	q := cloneQuery(r.URL.Query())
	q.Set("lang", lang)
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

	switchZH, switchEN := buildLangSwitchURLs(q)
	errMsg := errMessage(L, q.Get("err"))
	okMsg := ""
	if q.Get("ok") == "disconnect" {
		okMsg = L.OkDisconnect
	}

	data := struct {
		L            uiLocale
		HintMain     template.HTML
		HintWarn     template.HTML
		HintActive   template.HTML
		HintNeedPort template.HTML
		Users        []store.User
		Active       []registry.RemoteForward
		ErrMsg       string
		OKMsg        string
		RedirQuery   string
		PageSize     int
		UserPager    pagerInfo
		ActivePager  pagerInfo
		SwitchZH     string
		SwitchEN     string
	}{
		L:            L,
		HintMain:     template.HTML(L.HintMainHTML),
		HintWarn:     template.HTML(L.HintWarnHTML),
		HintActive:   template.HTML(L.HintActiveHTML),
		HintNeedPort: template.HTML(L.HintNeedPort),
		Users:        userSlice,
		Active:       activeSlice,
		ErrMsg:       errMsg,
		OKMsg:        okMsg,
		RedirQuery:   q.Encode(),
		PageSize:     listPageSize,
		UserPager:    buildPager(q, "upage", upage, uPages, uTotal),
		ActivePager:  buildPager(q, "apage", apage, aPages, aTotal),
		SwitchZH:     switchZH,
		SwitchEN:     switchEN,
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
	"jsQuote":  jsQuote,
}).Parse(`<!DOCTYPE html>
<html lang="{{.L.HTMLLang}}">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>{{.L.Title}}</title>
	<style>
		body { font-family: system-ui, sans-serif; max-width: 900px; margin: 2rem auto; padding: 0 1rem; color: #1a1a1a; }
		.hdr { display: flex; flex-wrap: wrap; align-items: flex-start; justify-content: space-between; gap: 0.75rem; }
		h1 { font-size: 1.4rem; margin: 0; }
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
		.lang-switch { font-size: 0.9rem; margin: 0; }
		.lang-switch a { color: #0b57d0; }
	</style>
</head>
<body>
	<div class="hdr">
		<h1>{{.L.Heading}}</h1>
		<p class="lang-switch"><a href="{{.SwitchZH}}">{{.L.LangZH}}</a> · <a href="{{.SwitchEN}}">{{.L.LangEN}}</a></p>
	</div>
	<p class="hint">{{.HintMain}}</p>
	<p class="hint warn">{{.HintWarn}}</p>
	{{if .ErrMsg}}<p class="err">{{.ErrMsg}}</p>{{end}}
	{{if .OKMsg}}<p class="okmsg">{{.OKMsg}}</p>{{end}}

	<h2>{{.L.H2Active}}</h2>
	<p class="hint">{{.HintActive}}</p>
	<table>
		<tr><th>{{.L.ThUser}}</th><th>{{.L.ThListenPort}}</th><th>{{.L.ThClientAddr}}</th><th>{{.L.ThConnID}}</th><th>{{.L.ThSince}}</th><th>{{.L.ThRate}}</th><th>{{.L.ThTraffic}}</th><th>{{.L.ThAction}}</th></tr>
		{{range .Active}}
		<tr>
			<td>{{.Username}}</td>
			<td class="mono">{{.Port}}</td>
			<td class="mono">{{.ClientAddr}}</td>
			<td class="mono">{{.ConnID}}</td>
			<td class="mono">{{.Since.Format "2006-01-02 15:04:05"}}</td>
			<td class="mono" title="{{.L.RateColTitle}}">{{.L.RateIn}} {{fmtBps .RateRxBps}}<br>{{.L.RateOut}} {{fmtBps .RateTxBps}}</td>
			<td class="mono">{{.L.TrafficIn}} {{fmtBytes .BytesRx}}<br>{{.L.TrafficOut}} {{fmtBytes .BytesTx}}</td>
			<td>
				<form method="post" action="/session/disconnect" style="margin:0" onsubmit="return confirm({{jsQuote $.L.ConfirmDisconnect}});">
					<input type="hidden" name="conn_id" value="{{.ConnID}}">
					<input type="hidden" name="redir" value="{{$.RedirQuery}}">
					<button type="submit">{{.L.BtnDisconnect}}</button>
				</form>
			</td>
		</tr>
		{{else}}
		<tr><td colspan="8">{{.L.NoActive}}</td></tr>
		{{end}}
	</table>
	{{if .ActivePager.Show}}
	<p class="pager">{{printf .L.PagerTotalActive .ActivePager.Total}}；{{printf .L.PagerPerPage .PageSize}}。
		{{if .ActivePager.PrevURL}}<a href="{{.ActivePager.PrevURL}}">{{.L.PrevPage}}</a>{{else}}<span class="disabled">{{.L.PrevPage}}</span>{{end}}
		{{printf .L.PageNofM .ActivePager.Page .ActivePager.TotalPages}}
		{{if .ActivePager.NextURL}}<a href="{{.ActivePager.NextURL}}">{{.L.NextPage}}</a>{{else}}<span class="disabled">{{.L.NextPage}}</span>{{end}}
	</p>
	{{end}}

	<h2>{{.L.H2Users}}</h2>
	<table>
		<tr><th>{{.L.ThUserAccount}}</th><th>{{.L.ThPassword}}</th><th>{{.L.ThTrafficSum}}</th><th>{{.L.ThFwdPorts}}</th><th>{{.L.ThEdit}}</th><th>{{.L.ThDelete}}</th></tr>
		{{range .Users}}
		<tr>
			<td class="mono">{{.Username}}</td>
			<td class="mono">{{if .StoredPassword}}{{.StoredPassword}}{{else}}<em title="{{$.L.NoPasswordTitle}}">—</em>{{end}}</td>
			<td class="mono" title="{{$.L.TrafficSumTitle}}">{{$.L.TrafficIn}} {{fmtBytes .TrafficRxTotal}}<br>{{$.L.TrafficOut}} {{fmtBytes .TrafficTxTotal}}</td>
			<td class="mono">{{range $i, $p := .ForwardPorts}}{{if $i}}, {{end}}{{$p}}{{else}}<em>{{$.L.NoPorts}}</em>{{end}}</td>
			<td>
				<details>
					<summary>{{.L.EditSummary}}</summary>
					<form method="post" action="/user/update" style="margin-top:.5rem;">
						<input type="hidden" name="id" value="{{.ID}}">
						<label>{{.L.LabelNewPwd}}</label>
						<input type="password" name="password" autocomplete="new-password">
						<label>{{.L.LabelFwdPorts}}</label>
						<textarea name="forward_ports">{{range $i, $p := .ForwardPorts}}{{if $i}},{{end}}{{$p}}{{end}}</textarea>
						<button type="submit">{{.L.Save}}</button>
					</form>
				</details>
			</td>
			<td>
				<form method="post" action="/user/delete" onsubmit="return confirm({{jsQuote (printf .L.ConfirmDelete .Username)}});">
					<input type="hidden" name="id" value="{{.ID}}">
					<button type="submit">{{.L.Delete}}</button>
				</form>
			</td>
		</tr>
		{{else}}
		<tr><td colspan="6">{{.L.NoUsersYet}}</td></tr>
		{{end}}
	</table>
	{{if .UserPager.Show}}
	<p class="pager">{{printf .L.PagerTotalUsers .UserPager.Total}}；{{printf .L.PagerPerPage .PageSize}}。
		{{if .UserPager.PrevURL}}<a href="{{.UserPager.PrevURL}}">{{.L.PrevPage}}</a>{{else}}<span class="disabled">{{.L.PrevPage}}</span>{{end}}
		{{printf .L.PageNofM .UserPager.Page .UserPager.TotalPages}}
		{{if .UserPager.NextURL}}<a href="{{.UserPager.NextURL}}">{{.L.NextPage}}</a>{{else}}<span class="disabled">{{.L.NextPage}}</span>{{end}}
	</p>
	{{end}}

	<h2>{{.L.H2Add}}</h2>
	<form method="post" action="/user/create">
		<label>{{.L.LabelUsername}}</label>
		<input type="text" name="username" required autocomplete="username">
		<label>{{.L.LabelPassword}}</label>
		<input type="password" name="password" required autocomplete="new-password">
		<label>{{.L.LabelFwdPortsAdd}}</label>
		<textarea name="forward_ports" placeholder="{{.L.PlaceholderPorts}}"></textarea>
		<p class="hint">{{.HintNeedPort}}</p>
		<button type="submit">{{.L.Create}}</button>
	</form>
</body>
</html>
`))
