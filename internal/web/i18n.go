package web

import (
	"net/http"
	"net/url"
	"strings"
)

const langCookieName = "lang"

func langFromRequest(r *http.Request) string {
	if l := strings.TrimSpace(r.URL.Query().Get("lang")); l == "en" || l == "zh" {
		return l
	}
	if c, err := r.Cookie(langCookieName); err == nil && (c.Value == "en" || c.Value == "zh") {
		return c.Value
	}
	return "zh"
}

// applyLangCookie sets the lang cookie when the user opens / with ?lang=zh|en.
func applyLangCookie(w http.ResponseWriter, r *http.Request) string {
	if r.Method == http.MethodGet && r.URL.Path == "/" {
		if l := strings.TrimSpace(r.URL.Query().Get("lang")); l == "en" || l == "zh" {
			http.SetCookie(w, &http.Cookie{
				Name:     langCookieName,
				Value:    l,
				Path:     "/",
				MaxAge:   365 * 24 * 3600,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})
			return l
		}
	}
	return langFromRequest(r)
}

func homeRedirect(w http.ResponseWriter, r *http.Request, q url.Values) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("lang", langFromRequest(r))
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

func redirectAfterDisconnect(w http.ResponseWriter, r *http.Request, redirRaw string) {
	q := url.Values{}
	q.Set("ok", "disconnect")
	if redirRaw != "" {
		rq, err := url.ParseQuery(redirRaw)
		if err == nil {
			for k, vs := range rq {
				for _, v := range vs {
					q.Add(k, v)
				}
			}
		}
	}
	q.Set("lang", langFromRequest(r))
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

func buildLangSwitchURLs(q url.Values) (switchZH, switchEN string) {
	z := cloneQuery(q)
	z.Set("lang", "zh")
	switchZH = "/?" + z.Encode()
	e := cloneQuery(q)
	e.Set("lang", "en")
	switchEN = "/?" + e.Encode()
	return switchZH, switchEN
}

type uiLocale struct {
	HTMLLang string

	Title        string
	Heading      string
	HintMainHTML string // safe HTML: <strong>, <code>
	HintWarnHTML string

	ErrUnknown        string
	ErrMissing        string
	ErrCreate         string
	ErrID             string
	ErrUpdate         string
	ErrBadConn        string
	ErrDisconnectGone string
	OkDisconnect      string

	H2Active         string
	HintActiveHTML   string
	ThUser           string
	ThListenPort     string
	ThClientAddr     string
	ThConnID         string
	ThSince          string
	ThRate           string
	ThTraffic        string
	ThAction         string
	BtnDisconnect    string
	RateIn           string
	RateOut          string
	TrafficIn        string
	TrafficOut       string
	RateColTitle     string
	NoActive         string
	PagerTotalActive string
	PagerTotalUsers  string
	PagerPerPage     string
	PrevPage         string
	NextPage         string
	PageNofM         string // "第 x / y 页" vs "Page x / y"

	ConfirmDisconnect string

	H2Users         string
	ThUserAccount   string
	ThPassword      string
	ThTrafficSum    string
	ThFwdPorts      string
	ThEdit          string
	ThDelete        string
	NoPasswordTitle string
	TrafficSumTitle string
	NoPorts         string
	EditSummary     string
	LabelNewPwd     string
	LabelFwdPorts   string
	Save            string
	ConfirmDelete   string
	Delete          string
	NoUsersYet      string

	H2Add            string
	LabelUsername    string
	LabelPassword    string
	LabelFwdPortsAdd string
	PlaceholderPorts string
	HintNeedPort     string
	Create           string

	LangZH string
	LangEN string
}

func localeFor(lang string) uiLocale {
	if lang == "en" {
		return uiLocale{
			HTMLLang:     "en",
			Title:        "SSH forward admin",
			Heading:      "SSH TCP forwarding",
			HintMainHTML: `The <strong>allowed remote forward ports</strong> below apply to <code>ssh -R</code> (server listen ports in <code>-R port:host:hostport</code>); they must match what you configure here. Client <code>-L</code> and SOCKS <code>-D</code> use <code>direct-tcpip</code>; <code>-allow-dynamic-forward</code> defaults to <strong>on</strong> (set <code>false</code> and omit <code>-allow-local-forward</code> to disable).`,
			HintWarnHTML: `Passwords in the list are stored <strong>in plaintext</strong> in the database for convenience. Do not expose the admin UI to the internet; do not commit <code>app.db</code>.`,

			ErrUnknown:        "Something went wrong (code: %s).",
			ErrMissing:        "Username and password are required.",
			ErrCreate:         "Could not create the user (duplicate name or invalid data).",
			ErrID:             "Invalid user id.",
			ErrUpdate:         "Could not update the user.",
			ErrBadConn:        "Invalid session id.",
			ErrDisconnectGone: "That session is no longer active.",
			OkDisconnect:      "The SSH client was disconnected (all its remote forwards were closed).",

			H2Active:         "Active remote forwards (-R)",
			HintActiveHTML:   `“In / out” is relative to traffic on the <strong>local listen port</strong>: in = bytes from peers into SSH, out = bytes to peers. Current speed is a 1-second average; session totals are for this SSH connection.`,
			ThUser:           "SSH user",
			ThListenPort:     "Listen port",
			ThClientAddr:     "Client address",
			ThConnID:         "Session ID",
			ThSince:          "Since",
			ThRate:           "Current speed",
			ThTraffic:        "Session traffic",
			ThAction:         "Action",
			BtnDisconnect:    "Disconnect",
			RateIn:           "In",
			RateOut:          "Out",
			TrafficIn:        "In",
			TrafficOut:       "Out",
			RateColTitle:     "In = peer→SSH, out = SSH→peer",
			NoActive:         "No active listeners",
			PagerTotalActive: "Total %d",
			PagerTotalUsers:  "%d users total",
			PagerPerPage:     "%d per page",
			PrevPage:         "Prev",
			NextPage:         "Next",
			PageNofM:         "Page %d / %d",

			ConfirmDisconnect: "Disconnect this SSH client? All -R forwards on that client will stop.",

			H2Users:         "SSH accounts and allowed ports",
			ThUserAccount:   "User",
			ThPassword:      "Password",
			ThTrafficSum:    "Traffic (all time)",
			ThFwdPorts:      "Allowed remote ports",
			ThEdit:          "Edit",
			ThDelete:        "Delete",
			NoPasswordTitle: "Created before this feature, or password never saved here",
			TrafficSumTitle: "Cumulative after each SSH session ends (forwarded sockets only)",
			NoPorts:         "none (-R denied)",
			EditSummary:     "Edit",
			LabelNewPwd:     "New password (leave blank to keep)",
			LabelFwdPorts:   "Allowed remote forward ports (comma-separated)",
			Save:            "Save",
			ConfirmDelete:   "Delete user %s?",
			Delete:          "Delete",
			NoUsersYet:      "No users yet — add one below",

			H2Add:            "Add user",
			LabelUsername:    "Username",
			LabelPassword:    "Password",
			LabelFwdPortsAdd: "Allowed remote forward ports (comma-separated, e.g. 8080,8443)",
			PlaceholderPorts: "8080,9000",
			HintNeedPort:     "At least one port is required for <code>ssh -R</code> to be allowed.",
			Create:           "Create",

			LangZH: "中文",
			LangEN: "English",
		}
	}
	return uiLocale{
		HTMLLang:     "zh-CN",
		Title:        "SSH 转发管理",
		Heading:      "SSH 端口转发服务",
		HintMainHTML: `下方「允许的远程转发端口」用于 <code>ssh -R</code>（即 <code>-R 端口:目标:目标端口</code> 中的服务端监听端口，须与此处登记一致）。客户端 <code>-L</code> 与 SOCKS <code>-D</code> 走 <code>direct-tcpip</code>；<code>-allow-dynamic-forward</code> 默认<strong>开启</strong>（若需关闭可设为 <code>false</code> 且勿开启 <code>-allow-local-forward</code>）。`,
		HintWarnHTML: `列表中的密码以<strong>明文</strong>写入数据库便于查看；请勿对公网暴露管理页，勿提交 <code>app.db</code>。`,

		ErrUnknown:        "操作未完成（错误代码：%s）",
		ErrMissing:        "请填写用户名和密码。",
		ErrCreate:         "无法创建用户（可能重名或数据无效）。",
		ErrID:             "用户编号无效。",
		ErrUpdate:         "无法更新用户。",
		ErrBadConn:        "会话编号无效。",
		ErrDisconnectGone: "该会话已不存在或已结束。",
		OkDisconnect:      "已断开该客户端的 SSH 连接（其全部远程转发已结束）。",

		H2Active:         "当前远程转发监听（-R）",
		HintActiveHTML:   `「入/出」相对<strong>本机监听端口</strong>上的流量：入=外部连接写入，出=写回外部。当前网速为最近 1 秒平均；本会话流量为当前 SSH 连接上该会话累计。`,
		ThUser:           "SSH 用户",
		ThListenPort:     "监听端口",
		ThClientAddr:     "客户端地址",
		ThConnID:         "会话 ID",
		ThSince:          "开始时间",
		ThRate:           "当前网速",
		ThTraffic:        "本会话流量",
		ThAction:         "操作",
		BtnDisconnect:    "断开",
		RateIn:           "入",
		RateOut:          "出",
		TrafficIn:        "入",
		TrafficOut:       "出",
		RateColTitle:     "入站=外部→SSH，出站=SSH→外部",
		NoActive:         "暂无活跃监听",
		PagerTotalActive: "共 %d 条",
		PagerTotalUsers:  "共 %d 个用户",
		PagerPerPage:     "每页 %d 条",
		PrevPage:         "上一页",
		NextPage:         "下一页",
		PageNofM:         "第 %d / %d 页",

		ConfirmDisconnect: "断开此 SSH 客户端？同一客户端上所有 -R 将一并结束。",

		H2Users:         "SSH 账号与允许端口",
		ThUserAccount:   "用户",
		ThPassword:      "密码",
		ThTrafficSum:    "历史流量汇总",
		ThFwdPorts:      "允许的远程转发端口",
		ThEdit:          "更新",
		ThDelete:        "删除",
		NoPasswordTitle: "在该功能上线前创建的用户，或从未通过本页保存过密码",
		TrafficSumTitle: "历次 SSH 会话结束后累计（按转发套接字统计）",
		NoPorts:         "无（禁止 -R）",
		EditSummary:     "编辑",
		LabelNewPwd:     "新密码（留空则不修改）",
		LabelFwdPorts:   "允许的远程转发端口（逗号分隔）",
		Save:            "保存",
		ConfirmDelete:   "删除用户 %s？",
		Delete:          "删除",
		NoUsersYet:      "尚无用户，请在下方添加",

		H2Add:            "添加用户",
		LabelUsername:    "用户名",
		LabelPassword:    "密码",
		LabelFwdPortsAdd: "允许的远程转发端口（逗号分隔，例如 8080,8443）",
		PlaceholderPorts: "8080,9000",
		HintNeedPort:     "至少需要登记一个端口，该用户才能使用 <code>ssh -R</code>。",
		Create:           "创建",

		LangZH: "中文",
		LangEN: "English",
	}
}

func errMessage(L uiLocale, code string) string {
	switch code {
	case "missing":
		return L.ErrMissing
	case "create":
		return L.ErrCreate
	case "id":
		return L.ErrID
	case "update":
		return L.ErrUpdate
	case "badconn":
		return L.ErrBadConn
	case "disconnect_gone":
		return L.ErrDisconnectGone
	default:
		if code == "" {
			return ""
		}
		return strings.ReplaceAll(L.ErrUnknown, "%s", code)
	}
}
