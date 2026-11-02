/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
	"golang.org/x/crypto/bcrypt"
)

const sessionCookie = "uruflow_session"
const sessionLifetime = 8 * time.Hour
const sessionIdleLifetime = 30 * time.Minute
const bootstrapFilename = "web-setup-token"

type session = storage.ConsoleSession
type attempt struct {
	Count int
	Until time.Time
}
type authentication struct {
	mu            sync.Mutex
	sessions      map[[32]byte]session
	attempts      map[string]attempt
	store         storage.ConsoleStore
	bootstrap     string
	bootstrapPath string
	secureCookies bool
	dummyHash     []byte
}

func newAuthentication(store storage.ConsoleStore, dataDir string, secure bool) (*authentication, error) {
	auth := &authentication{store: store, sessions: map[[32]byte]session{}, attempts: map[string]attempt{}, bootstrapPath: filepath.Join(dataDir, bootstrapFilename), secureCookies: secure}
	persisted, err := store.ListConsoleSessions()
	if err != nil {
		return nil, err
	}
	for _, item := range persisted {
		if expiredSession(item, time.Now()) {
			if err := store.DeleteConsoleSession(item.Hash); err != nil {
				return nil, err
			}
			continue
		}
		auth.sessions[item.Hash] = item
	}
	users, err := store.ListConsoleUsers()
	if err != nil {
		return nil, err
	}
	if len(users) == 0 {
		info, statErr := os.Lstat(auth.bootstrapPath)
		if statErr == nil && (!info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0) {
			return nil, errors.New("web setup token must be a private regular file (mode 0600)")
		}
		token, err := os.ReadFile(auth.bootstrapPath)
		if errors.Is(err, os.ErrNotExist) {
			material, err := randomToken()
			if err != nil {
				return nil, err
			}
			file, err := os.OpenFile(auth.bootstrapPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return nil, err
			}
			if _, err = file.WriteString(material + "\n"); err != nil {
				file.Close()
				return nil, err
			}
			if err = file.Close(); err != nil {
				return nil, err
			}
			token = []byte(material)
		} else if err != nil {
			return nil, err
		}
		auth.bootstrap = strings.TrimSpace(string(token))
		if len(auth.bootstrap) < 32 {
			return nil, errors.New("invalid web setup token file")
		}
	}
	auth.dummyHash, err = bcrypt.GenerateFromPassword([]byte("unused-authentication-timing-value"), bcrypt.DefaultCost)
	return auth, err
}
func randomToken() (string, error) {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data[:]), nil
}
func (a *authentication) createSession(w http.ResponseWriter, r *http.Request, u *storage.ConsoleUser) (session, error) {
	token, err := randomToken()
	if err != nil {
		return session{}, err
	}
	csrf, err := randomToken()
	if err != nil {
		return session{}, err
	}
	now := time.Now()
	s := session{Hash: sha256.Sum256([]byte(token)), Username: u.Username, CSRF: csrf, Expires: now.Add(sessionLifetime), LastActive: now, Credentials: credentialVersion(u)}
	a.mu.Lock()
	for key, old := range a.sessions {
		if expiredSession(old, time.Now()) {
			if err := a.store.DeleteConsoleSession(key); err != nil {
				a.mu.Unlock()
				return session{}, err
			}
			delete(a.sessions, key)
		}
	}
	count := 0
	var oldestKey [32]byte
	var oldest time.Time
	for key, old := range a.sessions {
		if old.Username == u.Username {
			count++
			if oldest.IsZero() || old.LastActive.Before(oldest) {
				oldestKey = key
				oldest = old.LastActive
			}
		}
	}
	if count >= 16 {
		if err := a.store.DeleteConsoleSession(oldestKey); err != nil {
			a.mu.Unlock()
			return session{}, err
		}
		delete(a.sessions, oldestKey)
	}
	if len(a.sessions) >= 4096 {
		a.mu.Unlock()
		return session{}, errors.New("session capacity reached")
	}
	if err := a.store.SaveConsoleSession(s); err != nil {
		a.mu.Unlock()
		return session{}, err
	}
	a.sessions[s.Hash] = s
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: a.secureCookies || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: int(sessionLifetime.Seconds())})
	return s, nil
}
func (a *authentication) identify(r *http.Request) (session, *storage.ConsoleUser, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return session{}, nil, err
	}
	key := sha256.Sum256([]byte(cookie.Value))
	a.mu.Lock()
	s, ok := a.sessions[key]
	if ok && expiredSession(s, time.Now()) {
		delete(a.sessions, key)
		ok = false
	}
	a.mu.Unlock()
	if !ok {
		return session{}, nil, errors.New("session expired")
	}
	u, err := a.store.GetConsoleUser(s.Username)
	if err == nil && s.Credentials != credentialVersion(u) {
		err = errors.New("credentials changed")
	}
	return s, u, err
}
func (a *authentication) throttled(r *http.Request) bool {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	return a.throttledKey("ip:" + ip)
}
func (a *authentication) throttledKey(key string) bool {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	for oldKey, item := range a.attempts {
		if now.After(item.Until) {
			delete(a.attempts, oldKey)
		}
	}
	item := a.attempts[key]
	if item.Count >= 8 && now.Before(item.Until) {
		return true
	}
	if len(a.attempts) >= 4096 && item.Count == 0 {
		return true
	}
	item.Count++
	if item.Until.IsZero() {
		item.Until = now.Add(10 * time.Minute)
	}
	a.attempts[key] = item
	return false
}
func (a *authentication) sessionStatus(c *gin.Context) {
	w, r := c.Writer, c.Request
	s, u, err := a.identify(r)
	if err != nil {
		a.mu.Lock()
		setup := a.bootstrap != ""
		a.mu.Unlock()
		writeJSON(w, 200, map[string]any{"authenticated": false, "setup_required": setup})
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "user": u, "csrf_token": s.CSRF})
}
func (a *authentication) login(c *gin.Context) {
	w, r := c.Writer, c.Request
	if a.throttled(r) {
		writeError(w, 429, "Too many sign-in attempts. Try again later.")
		return
	}
	var body struct{ Username, Password string }
	if !readJSON(w, r, &body) {
		return
	}
	if models.ValidResourceName(body.Username) && a.throttledKey("user:"+body.Username) {
		writeError(w, 429, "Too many sign-in attempts. Try again later.")
		return
	}
	u, err := a.store.GetConsoleUser(body.Username)
	hash := a.dummyHash
	if err == nil {
		hash = u.PasswordHash
	}
	matched := bcrypt.CompareHashAndPassword(hash, []byte(body.Password)) == nil && len(body.Password) <= 72
	if err != nil || !matched {
		actor := body.Username
		if !models.ValidResourceName(actor) {
			actor = "<invalid>"
		}
		_ = a.store.AppendAudit(&storage.AuditEntry{Actor: actor, Action: "session.login", Resource: "web-session", Outcome: "failed"})
		writeError(w, 401, "Username or password is incorrect.")
		return
	}
	if ip, _, splitErr := net.SplitHostPort(r.RemoteAddr); splitErr == nil {
		a.mu.Lock()
		delete(a.attempts, "ip:"+ip)
		delete(a.attempts, "user:"+u.Username)
		a.mu.Unlock()
	}
	if err := a.store.AppendAudit(&storage.AuditEntry{Actor: u.Username, Action: "session.login", Resource: "web-session", Outcome: "succeeded"}); err != nil {
		writeError(w, 503, "Authentication audit is unavailable. Try again later.")
		return
	}
	s, err := a.createSession(w, r, u)
	if err != nil {
		writeError(w, 503, "Could not create a session.")
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "user": u, "csrf_token": s.CSRF})
}
func (a *authentication) setup(c *gin.Context) {
	w, r := c.Writer, c.Request
	if a.throttled(r) {
		writeError(w, 429, "Too many setup attempts. Try again later.")
		return
	}
	var body struct{ Token, Username, Password string }
	if !readJSON(w, r, &body) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.bootstrap == "" {
		writeError(w, 409, "An administrator already exists.")
		return
	}
	if subtle.ConstantTimeCompare([]byte(body.Token), []byte(a.bootstrap)) != 1 {
		writeError(w, 403, "The setup token is incorrect.")
		return
	}
	if !validCredentials(body.Username, body.Password) {
		writeError(w, 400, "Use a lowercase resource name and a password of at least 15 characters (maximum 72 bytes).")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, 500, "Could not secure the password.")
		return
	}
	created, err := a.store.BootstrapConsoleUser(&storage.ConsoleUser{Username: body.Username, Role: "admin", PasswordHash: hash})
	if err != nil {
		writeError(w, 500, "Could not create the administrator.")
		return
	}
	if !created {
		a.bootstrap = ""
		writeError(w, 409, "An administrator already exists.")
		return
	}
	a.bootstrap = ""
	_ = os.Remove(a.bootstrapPath)
	writeJSON(w, 201, map[string]any{"created": true})
}
func validCredentials(username, password string) bool {
	return models.ValidResourceName(username) && validPassword(username, password)
}
func (a *authentication) logout(c *gin.Context) {
	w, r := c.Writer, c.Request
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		if err := a.store.DeleteConsoleSession(sha256.Sum256([]byte(cookie.Value))); err != nil {
			a.mu.Unlock()
			writeError(w, 503, "Could not end the session. Try again.")
			return
		}
		delete(a.sessions, sha256.Sum256([]byte(cookie.Value)))
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", HttpOnly: true, Secure: a.secureCookies || r.TLS != nil, SameSite: http.SameSiteStrictMode, MaxAge: -1})
	writeJSON(w, 200, map[string]bool{"signed_out": true})
}

func (a *authentication) revokeUser(username string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.store.DeleteConsoleUserSessions(username); err != nil {
		return err
	}
	for key, s := range a.sessions {
		if s.Username == username {
			delete(a.sessions, key)
		}
	}
	return nil
}

func validPassword(username, password string) bool {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 15 || len(password) > 72 {
		return false
	}
	normalized := strings.ToLower(password)
	if normalized == strings.ToLower(username) {
		return false
	}
	switch normalized {
	case "passwordpassword", "passwordpasswordpassword", "123456789012345", "1234567890123456", "qwertyuiopasdfgh", "uruflowpassword":
		return false
	}
	return true
}
func credentialVersion(u *storage.ConsoleUser) [32]byte {
	data, _ := json.Marshal(struct {
		Role     string
		Projects []string
		Hash     []byte
	}{u.Role, u.Projects, u.PasswordHash})
	return sha256.Sum256(data)
}
func expiredSession(s session, now time.Time) bool {
	return !now.Before(s.Expires) || !now.Before(s.LastActive.Add(sessionIdleLifetime))
}
func (a *authentication) refresh(c *gin.Context) {
	w, r := c.Writer, c.Request
	cookie, _ := r.Cookie(sessionCookie)
	key := sha256.Sum256([]byte(cookie.Value))
	a.mu.Lock()
	s, ok := a.sessions[key]
	if ok && !expiredSession(s, time.Now()) {
		s.LastActive = time.Now()
		if err := a.store.SaveConsoleSession(s); err != nil {
			a.mu.Unlock()
			writeError(w, 503, "Could not refresh the session. Try again.")
			return
		}
		a.sessions[key] = s
	} else {
		ok = false
	}
	a.mu.Unlock()
	if !ok {
		writeError(w, 401, "Sign in to continue.")
		return
	}
	writeJSON(w, 200, map[string]bool{"active": true})
}
