package middleware

import (
	"net/http"

	"github.com/gorilla/sessions"
)

const sessionName = "admin"
const sessionKey = "authenticated"

func NewSessionStore(secret string) *sessions.CookieStore {
	store := sessions.NewCookieStore([]byte(secret))
	store.Options = &sessions.Options{
		Path:     "/admin",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	return store
}

func RequireAdmin(store *sessions.CookieStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, _ := store.Get(r, sessionName)
		if auth, ok := sess.Values[sessionKey].(bool); !ok || !auth {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func SetAuthenticated(store *sessions.CookieStore, w http.ResponseWriter, r *http.Request) error {
	sess, _ := store.Get(r, sessionName)
	sess.Values[sessionKey] = true
	return sess.Save(r, w)
}

func Logout(store *sessions.CookieStore, w http.ResponseWriter, r *http.Request) error {
	sess, _ := store.Get(r, sessionName)
	sess.Values[sessionKey] = false
	sess.Options.MaxAge = -1
	return sess.Save(r, w)
}
