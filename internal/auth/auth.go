package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const sessionCookie = "colday_session"
const sessionMaxAge = 7 * 24 * time.Hour

type Auth struct {
	Password     string
	SessionKey   []byte
	CookieSecure bool
}

func New(password string, sessionKey []byte, cookieSecure bool) *Auth {
	if len(sessionKey) == 0 {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic("auth: failed to generate session key: " + err.Error())
		}
		sessionKey = key
	}
	return &Auth{
		Password:     password,
		SessionKey:   sessionKey,
		CookieSecure: cookieSecure,
	}
}

func (a *Auth) signSession(expires int64) string {
	expStr := strconv.FormatInt(expires, 10)
	mac := hmac.New(sha256.New, a.SessionKey)
	mac.Write([]byte(expStr))
	return base64.RawURLEncoding.EncodeToString([]byte(expStr)) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *Auth) verifySession(value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	expBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, a.SessionKey)
	mac.Write(expBytes)
	if subtle.ConstantTimeCompare(sig, mac.Sum(nil)) != 1 {
		return false
	}
	exp, err := strconv.ParseInt(string(expBytes), 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func (a *Auth) SessionCookieValue() string {
	return a.signSession(time.Now().Add(sessionMaxAge).Unix())
}

func (a *Auth) SetSessionCookie(w http.ResponseWriter) {
	value := a.SessionCookieValue()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.CookieSecure,
		MaxAge:   int(sessionMaxAge.Seconds()),
	})
}

func (a *Auth) ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   a.CookieSecure,
		MaxAge:   -1,
	})
}

func (a *Auth) IsAuthed(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return a.verifySession(c.Value)
}

func (a *Auth) CheckPassword(password string) bool {
	return subtle.ConstantTimeCompare([]byte(password), []byte(a.Password)) == 1
}
