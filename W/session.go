package W

import (
	"github.com/kokizzu/gotro/M"
	"github.com/kokizzu/gotro/S"
	"github.com/kokizzu/gotro/T"
	"github.com/valyala/fasthttp"
	"time"
)

type SessionConnector interface {
	// delete key
	Del(key string)
	// get remaining lifespan in seconds
	Expiry(key string) int64
	// set string with remaining lifespan in seconds
	FadeStr(key, val string, ttl int64)
	// set integer with remaining lifespan in seconds
	FadeInt(key string, val int64, ttl int64)
	// set json with remaining lifespan in seconds
	FadeMSX(key string, val M.SX, ttl int64)
	// get string
	GetStr(key string) string
	// get integer
	GetInt(key string) int64
	// get string
	GetMSX(key string) M.SX
	// increment
	Inc(key string) int64
	// set string
	SetStr(key, val string)
	// set integer
	SetInt(key string, val int64)
	// set json
	SetMSX(key string, val M.SX)
}

var SESS_KEY = `SK`
var EXPIRE_SEC int64
var RENEW_SEC int64

const NS2SEC = 1000 * 1000 * 1000

type Session struct {
	UserAgent string
	IpAddr    string
	Key       string
	M.SX
	Changed bool
}

func (s *Session) Logout() {
	Sessions.Del(s.Key)
	s.Key = ``
	s.SX = M.SX{}
	s.Changed = true
}

func (s *Session) RandomKey() {
	for {
		s.Key = s.StateCSRF() + S.RandomCB63(8)
		if Sessions.GetStr(s.Key) == `` {
			break // no collision
		}
	}
	s.Changed = true
}

func (s *Session) Login(val M.SX) {
	if s.Key == `` {
		s.RandomKey()
	}
	val[`ip_addr`] = s.IpAddr
	val[`user_agent`] = s.UserAgent
	val[`login_at`] = T.Epoch()
	s.SX = val
	Sessions.FadeMSX(s.Key, val, EXPIRE_SEC)
}

// should be called after receiving request
func (s *Session) Load(ctx *Context) {
	r := ctx.RequestCtx
	h := r.Request.Header
	s.UserAgent = string(h.UserAgent())
	s.IpAddr = r.RemoteAddr().String()
	cookie := string(h.Cookie(SESS_KEY))
	if cookie == `` {
		s.SX = M.SX{}
	} else if !S.StartsWith(cookie, s.StateCSRF()) {
		s.Logout() // possible incorrect cookie stealing
	} else {
		s.Key = cookie
		s.SX = Sessions.GetMSX(s.Key)
		if len(s.SX) == 0 {
			s.Logout() // possible expired cookie
		}
	}
}

// should be called before writing response
func (s *Session) Save(ctx *Context) {
	if s.Changed {
		rem := Sessions.Expiry(s.Key)
		expiration := time.Now().Add(time.Second * time.Duration(rem))
		cookie := &fasthttp.Cookie{}
		cookie.SetKey(SESS_KEY)
		cookie.SetValue(s.Key)
		cookie.SetExpire(expiration)
		ctx.Response.Header.SetCookie(cookie)
	}
}

func (s *Session) StateCSRF() string {
	return S.HashPassword(s.UserAgent) + `|`
}

func (s *Session) Touch() {
	if s.Key == `` {
		return
	}
	if Sessions.Expiry(s.Key) < RENEW_SEC {
		s.SX[`renew_at`] = T.Epoch()
		s.Changed = true
		Sessions.FadeMSX(s.Key, s.SX, EXPIRE_SEC)
	}
}

func (s *Session) String() string {
	if len(s.SX) == 0 {
		return ``
	}
	return s.SX.Pretty(` | `)
}

func InitSession(sess_key string, expire_ns, renew_ns time.Duration, conn SessionConnector) {
	SESS_KEY = S.IfEmpty(sess_key, SESS_KEY)
	EXPIRE_SEC = int64(expire_ns / NS2SEC)
	RENEW_SEC = int64(renew_ns / NS2SEC)
	Sessions = conn
}