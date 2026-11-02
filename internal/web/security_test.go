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
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mustafanass/uruflow/internal/activity"
	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/observe"
	"github.com/mustafanass/uruflow/internal/ops"
	"github.com/mustafanass/uruflow/internal/projects"
	"github.com/mustafanass/uruflow/internal/storage"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"golang.org/x/crypto/bcrypt"
)

const testPassword = "correct horse battery staple"

type fixtureOperator struct {
	Operator
	calls int
}

func (f *fixtureOperator) Release(id string) (*models.Release, error) {
	return &models.Release{ID: id, Project: "private-project"}, nil
}
func (f *fixtureOperator) Container(agent, id string) (*models.Container, error) {
	return &models.Container{AgentID: agent, ID: id, Project: "private-project"}, nil
}
func (f *fixtureOperator) Alert(id string) (*models.Alert, error) {
	return &models.Alert{ID: id, Project: "private-project"}, nil
}
func (f *fixtureOperator) RecentAlerts() ([]models.Alert, error) {
	now := time.Now()
	return []models.Alert{
		{ID: "allowed-alert", Project: "assigned-project", Resolved: true, ResolvedAt: &now},
		{ID: "private-alert", Project: "private-project"},
		{ID: "global-alert"},
	}, nil
}
func (f *fixtureOperator) StopProject(string) error { f.calls++; return nil }
func (f *fixtureOperator) ProjectDocument(string, bool) (ops.Document, error) {
	return ops.Document{Content: "private-secret"}, nil
}

func newConsole(t *testing.T, operator Operator, seed bool) (*Handler, *sqlite.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := sqlite.New(filepath.Join(dir, "console.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if seed {
		for _, u := range []storage.ConsoleUser{{Username: "admin", Role: "admin"}, {Username: "operator", Role: "operator", Projects: []string{"assigned-project"}}, {Username: "viewer", Role: "viewer", Projects: []string{"assigned-project"}}} {
			u.PasswordHash, _ = bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.MinCost)
			if err = store.CreateConsoleUser(&u); err != nil {
				t.Fatal(err)
			}
		}
	}
	h, err := New(operator, store, dir, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h, store
}
func request(h http.Handler, method, path, body string, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "https://workspace.test"+path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.1:1234"
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if csrf != "" {
		r.Header.Set("X-CSRF-Token", csrf)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func signIn(t *testing.T, h *Handler, user string) (*http.Cookie, string) {
	t.Helper()
	w := request(h, "POST", "/api/session", `{"username":"`+user+`","password":"`+testPassword+`"}`, nil, "")
	if w.Code != 200 {
		t.Fatalf("login %s: %d %s", user, w.Code, w.Body.String())
	}
	var data struct {
		CSRF string `json:"csrf_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("expected one session cookie")
	}
	return cookies[0], data.CSRF
}
func TestEveryResourceEndpointRequiresAuthentication(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	routes := []string{"GET /api/snapshot", "GET /api/stream", "GET /api/trust-root", "POST /api/agents", "DELETE /api/agents/a", "POST /api/projects", "POST /api/projects/reload", "GET /api/projects/p/configuration", "GET /api/projects/p/variables", "PUT /api/projects/p/configuration", "PUT /api/projects/p/variables", "POST /api/projects/p/deploy", "POST /api/projects/p/rollback", "POST /api/projects/p/stop", "GET /api/projects/p/releases", "GET /api/releases/r", "GET /api/releases/r/logs", "GET /api/containers/a/c/logs", "GET /api/images", "GET /api/registry/catalog", "GET /api/images/details", "DELETE /api/images", "POST /api/alerts/a/resolve", "GET /api/users", "POST /api/users", "DELETE /api/users/u", "PUT /api/users/u/role", "PUT /api/users/u/projects", "POST /api/account/password", "GET /api/audit", "DELETE /api/session", "POST /api/session/refresh", "PUT /api/users/u/password", "DELETE /api/account/sessions"}
	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			method, path, _ := strings.Cut(route, " ")
			w := request(h, method, path, `{}`, nil, "")
			if w.Code != 401 {
				t.Fatalf("got %d: %s", w.Code, w.Body.String())
			}
		})
	}
}
func TestRolesAndCSRFEnforcedAtHTTPBoundary(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	viewer, csrf := signIn(t, h, "viewer")
	for _, route := range []string{"GET /api/projects/assigned-project/configuration", "GET /api/projects/assigned-project/variables", "POST /api/projects/assigned-project/deploy", "POST /api/projects/assigned-project/rollback", "POST /api/projects/assigned-project/stop", "POST /api/projects", "POST /api/projects/reload", "POST /api/agents", "DELETE /api/agents/a", "GET /api/users", "POST /api/users", "DELETE /api/users/u", "PUT /api/users/u/role", "PUT /api/users/u/projects", "PUT /api/users/u/password", "DELETE /api/images", "POST /api/alerts/a/resolve", "GET /api/audit"} {
		method, path, _ := strings.Cut(route, " ")
		if w := request(h, method, path, `{}`, viewer, csrf); w.Code != 403 {
			t.Fatalf("viewer %s: %d %s", route, w.Code, w.Body.String())
		}
	}
	admin, csrf := signIn(t, h, "admin")
	for _, token := range []string{"", "wrong"} {
		if w := request(h, "POST", "/api/users", `{}`, admin, token); w.Code != 403 {
			t.Fatalf("CSRF accepted %q: %d", token, w.Code)
		}
	}
	if w := request(h, "POST", "/api/users", `{"username":"new-user","password":"correct horse battery staple","role":"viewer"}`, admin, csrf); w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}
	if w := request(h, "GET", "/api/users", "", admin, ""); strings.Contains(w.Body.String(), "password_hash") || strings.Contains(w.Body.String(), testPassword) || strings.Contains(w.Body.String(), "$2a$") {
		t.Fatal("credential disclosed")
	}
	if w := request(h, "POST", "/api/users", `{} {}`, admin, csrf); w.Code != 400 {
		t.Fatal("multiple JSON objects accepted")
	}
}
func TestProjectAssignmentsPreventObjectIDGuessing(t *testing.T) {
	backend := &fixtureOperator{}
	h, _ := newConsole(t, backend, true)
	cookie, csrf := signIn(t, h, "operator")
	for _, route := range []string{"GET /api/projects/private-project/configuration", "GET /api/projects/private-project/variables", "PUT /api/projects/private-project/configuration", "PUT /api/projects/private-project/variables", "POST /api/projects/private-project/deploy", "POST /api/projects/private-project/rollback", "POST /api/projects/private-project/stop", "GET /api/projects/private-project/releases", "GET /api/releases/private-id", "GET /api/releases/private-id/logs", "GET /api/containers/shared-agent/private-container/logs", "POST /api/alerts/private-alert/resolve"} {
		method, path, _ := strings.Cut(route, " ")
		w := request(h, method, path, `{}`, cookie, csrf)
		if w.Code != 404 || strings.Contains(w.Body.String(), "private-secret") {
			t.Fatalf("scope escaped %s: %d %s", route, w.Code, w.Body.String())
		}
	}
	for _, route := range []string{"POST /api/projects", "POST /api/projects/reload", "GET /api/images"} {
		method, path, _ := strings.Cut(route, " ")
		if w := request(h, method, path, `{}`, cookie, csrf); w.Code != 403 {
			t.Fatalf("fleet scope escaped %s: %d", route, w.Code)
		}
	}
	if backend.calls != 0 {
		t.Fatal("unauthorized mutation reached backend")
	}
	if w := request(h, "POST", "/api/projects/assigned-project/stop", `{}`, cookie, csrf); w.Code != 200 || backend.calls != 1 {
		t.Fatalf("allowed project operation denied: %d %s", w.Code, w.Body.String())
	}
}
func accessSnapshot() observe.Snapshot {
	return observe.Snapshot{At: time.Now(), Host: &models.Metrics{}, RegistryAddress: "private-registry", Problems: []string{"private-project-path"},
		Agents:     []models.Agent{{ID: "shared-agent"}, {ID: "private-agent"}},
		Projects:   []models.Project{{Name: "assigned-project", Runners: []string{"shared-agent"}}, {Name: "private-project", Runners: []string{"shared-agent", "private-agent"}}},
		Containers: []models.Container{{ID: "allowed-container", Project: "assigned-project", AgentID: "shared-agent"}, {ID: "private-container", Project: "private-project", AgentID: "shared-agent"}, {ID: "system-container", AgentID: "shared-agent"}},
		Releases:   []models.Release{{ID: "allowed-release", Project: "assigned-project"}, {ID: "private-release", Project: "private-project"}},
		Alerts:     []models.Alert{{ID: "allowed-alert", Project: "assigned-project"}, {ID: "private-alert", Project: "private-project"}, {ID: "private-global-alert", AgentID: "shared-agent"}},
		Activity:   []activity.Entry{{Sequence: 1, Operation: "allowed-release", Message: "allowed"}, {Sequence: 2, Operation: "private-release", Message: "private-project-activity"}, {Sequence: 3, Source: "shared-agent", Message: "private-project-unscoped-event"}},
		Graphs:     map[string]observe.ProjectGraphs{"assigned-project": {}, "private-project": {}}, ProjectStates: map[string]string{"assigned-project": "running", "private-project": "running"},
	}
}
func TestSnapshotAndLiveStreamNeverDiscloseOtherProjects(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	s := accessSnapshot()
	h.latest = &s
	cookie, _ := signIn(t, h, "viewer")
	w := request(h, "GET", "/api/snapshot", "", cookie, "")
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	if strings.Contains(w.Body.String(), "private-") || strings.Contains(w.Body.String(), "system-container") {
		t.Fatal("restricted snapshot leaked data: ", w.Body.String())
	}
	var visible observe.Snapshot
	if err := json.Unmarshal(w.Body.Bytes(), &visible); err != nil {
		t.Fatal(err)
	}
	if len(visible.Projects) != 1 || len(visible.Agents) != 1 || len(visible.Containers) != 1 || len(visible.Releases) != 1 || len(visible.Activity) != 1 || visible.Host != nil {
		t.Fatalf("incorrect scoped view: %+v", visible)
	}
	if len(s.Projects) != 2 || len(s.Containers) != 3 || len(s.Graphs) != 2 {
		t.Fatal("shared snapshot mutated")
	}
	server := httptest.NewTLSServer(h)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL+"/api/stream", nil)
	req.AddCookie(cookie)
	response, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	scanner := bufio.NewScanner(response.Body)
	found := false
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			if strings.Contains(line, "private-") {
				t.Fatal("live stream leaked another project")
			}
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no initial live snapshot")
	}
	h.auth.revokeUser("viewer")
	h.mu.Lock()
	for subscriber := range h.subscribers {
		subscriber <- s
	}
	h.mu.Unlock()
	expired := false
	for scanner.Scan() {
		if scanner.Text() == "event: session-expired" {
			expired = true
			break
		}
	}
	if !expired {
		t.Fatal("revoked live stream remained authorized")
	}
}
func TestBootstrapIsLocalOneTimeAndCredentialsAreHashed(t *testing.T) {
	h, store := newConsole(t, &fixtureOperator{}, false)
	info, err := os.Stat(h.SetupTokenPath())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("setup token permissions: %v %v", info, err)
	}
	token, err := os.ReadFile(h.SetupTokenPath())
	if err != nil {
		t.Fatal(err)
	}
	w := request(h, "POST", "/api/setup", `{"username":"admin","password":"`+testPassword+`","token":"incorrect"}`, nil, "")
	if w.Code != 403 {
		t.Fatal("setup without local token accepted")
	}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": testPassword, "token": strings.TrimSpace(string(token))})
	if w = request(h, "POST", "/api/setup", string(body), nil, ""); w.Code != 201 {
		t.Fatalf("setup: %d %s", w.Code, w.Body.String())
	}
	if _, err = os.Stat(h.SetupTokenPath()); !os.IsNotExist(err) {
		t.Fatal("setup token retained")
	}
	if w = request(h, "POST", "/api/setup", string(body), nil, ""); w.Code != 409 {
		t.Fatal("bootstrap reused")
	}
	user, err := store.GetConsoleUser("admin")
	if err != nil || bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(testPassword)) != nil {
		t.Fatal("password not hashed")
	}
	cookie, csrf := signIn(t, h, "admin")
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || csrf == "" {
		t.Fatalf("unsafe session cookie: %+v", cookie)
	}
	if w = request(h, "DELETE", "/api/session", `{}`, cookie, csrf); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w = request(h, "GET", "/api/snapshot", "", cookie, ""); w.Code != 401 {
		t.Fatal("logged out session accepted")
	}
}
func TestCrossSiteRequestsCannotLoginOrBootstrap(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	for _, path := range []string{"/api/session", "/api/setup"} {
		r := httptest.NewRequest("POST", "https://workspace.test"+path, strings.NewReader(`{}`))
		r.Header.Set("Origin", "https://attacker.test")
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatalf("cross-site %s accepted: %d", path, w.Code)
		}
	}
}
func TestSessionsExpireAndPermissionChangesInvalidateCredentials(t *testing.T) {
	h, store := newConsole(t, &fixtureOperator{}, true)
	for _, mode := range []string{"idle", "absolute", "password", "role", "projects", "deletion"} {
		t.Run(mode, func(t *testing.T) {
			u, err := store.GetConsoleUser("operator")
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "https://workspace.test", nil)
			_, err = h.auth.createSession(recorder, req, u)
			if err != nil {
				t.Fatal(err)
			}
			cookie := recorder.Result().Cookies()[0]
			key := sha256.Sum256([]byte(cookie.Value))
			switch mode {
			case "idle", "absolute":
				h.auth.mu.Lock()
				s := h.auth.sessions[key]
				if mode == "idle" {
					s.LastActive = time.Now().Add(-sessionIdleLifetime)
				} else {
					s.Expires = time.Now().Add(-time.Second)
				}
				h.auth.sessions[key] = s
				h.auth.mu.Unlock()
			case "password":
				hash, _ := bcrypt.GenerateFromPassword([]byte("another correct long password"), bcrypt.MinCost)
				err = store.ChangeConsolePassword(u.Username, u.PasswordHash, hash)
			case "role":
				err = store.ChangeConsoleRole(u.Username, "viewer")
			case "projects":
				err = store.ChangeConsoleProjects(u.Username, []string{"other-project"})
			case "deletion":
				err = store.DeleteConsoleUser(u.Username)
			}
			if err != nil {
				t.Fatal(err)
			}
			w := request(h, "GET", "/api/snapshot", "", cookie, "")
			if w.Code != 401 {
				t.Fatalf("%s invalidation failed: %d", mode, w.Code)
			}
		})
	}
}
func TestLoginRateLimitAndGenericErrors(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	for i := 0; i < 9; i++ {
		name := "admin"
		if i%2 == 1 {
			name = "does-not-exist"
		}
		w := request(h, "POST", "/api/session", `{"username":"`+name+`","password":"wrong"}`, nil, "")
		if i < 8 {
			if w.Code != 401 || !strings.Contains(w.Body.String(), "Username or password is incorrect") {
				t.Fatalf("login enumeration: %d %s", w.Code, w.Body.String())
			}
		} else if w.Code != 429 {
			t.Fatal("login rate limit missing")
		}
	}
}

type unavailableAudit struct{ storage.ConsoleStore }

func (unavailableAudit) AppendAudit(*storage.AuditEntry) error { return errors.New("unavailable") }
func TestMutationFailsClosedWhenAuditUnavailable(t *testing.T) {
	backend := &fixtureOperator{}
	h, store := newConsole(t, backend, true)
	cookie, csrf := signIn(t, h, "operator")
	h.accounts = unavailableAudit{store}
	w := request(h, "POST", "/api/projects/assigned-project/stop", `{}`, cookie, csrf)
	if w.Code != 503 || backend.calls != 0 {
		t.Fatalf("unaudited mutation ran: %d, calls %d", w.Code, backend.calls)
	}
}
func TestPasswordPolicyAcceptsPassphrasesAndCountsUnicode(t *testing.T) {
	for _, p := range []string{"short", strings.Repeat("界", 14), strings.Repeat("x", 73), "123456789012345", "passwordpassword"} {
		if validPassword("user", p) {
			t.Fatal("weak or oversized password accepted")
		}
	}
	if !validPassword("user", strings.Repeat("界", 15)) || !validPassword("user", testPassword) {
		t.Fatal("valid passphrase rejected")
	}
}

func TestResolvedConfigurationCannotEscapeAgentAndSecretPermissions(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	s := accessSnapshot()
	h.latest = &s
	u := &storage.ConsoleUser{Role: "operator", Projects: []string{"assigned-project"}}
	policy, err := h.documentPolicy(u, "assigned-project")
	if err != nil {
		t.Fatal(err)
	}
	loader := projects.NewLoader(t.TempDir(), func(name string) (*models.Agent, error) {
		return &models.Agent{ID: name, Name: name, Roles: []models.Role{models.RoleRunner}}, nil
	})
	path := filepath.Join(loader.Dir(), "assigned", "project.yaml")
	base := "runners: [shared-agent]\nservices:\n  api:\n    image: nginx@sha256:" + strings.Repeat("a", 64) + "\n"
	for _, test := range []struct{ name, configuration, variables string }{
		{"escaped secret", base + `env:
  TOKEN: "\x24{secret:private-project.KEY}"
`, ""},
		{"composed secret", base, "DOLLAR=$\nREFERENCE={secret:private-project.KEY}\nTOKEN=${DOLLAR}${REFERENCE}\n"},
		{"unassigned agent", strings.Replace(base, "shared-agent", "private-agent", 1), ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, err := loader.Preview(path, test.configuration, &test.variables)
			if err != nil {
				t.Fatal(err)
			}
			if err = policy(*p); !errors.Is(err, errForbidden) {
				t.Fatalf("escaped authorization: %v", err)
			}
		})
	}
	own := base + `env:
  TOKEN: "${secret:assigned-project.KEY}"
`
	variables := ""
	p, err := loader.Preview(path, own, &variables)
	if err != nil {
		t.Fatal(err)
	}
	if err = policy(*p); err != nil {
		t.Fatalf("project's own secret rejected: %v", err)
	}
}

func TestAccountManagementRevokesSessionsAndProtectsLastAdministrator(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	admin, csrf := signIn(t, h, "admin")
	viewer, _ := signIn(t, h, "viewer")
	w := request(h, "PUT", "/api/users/viewer/projects", `{"projects":["other-project"]}`, admin, csrf)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request(h, "GET", "/api/snapshot", "", viewer, ""); w.Code != 401 {
		t.Fatal("project changes left an old session valid")
	}
	viewer, _ = signIn(t, h, "viewer")
	if w = request(h, "PUT", "/api/users/viewer/password", `{"password":"a replacement test passphrase"}`, admin, csrf); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if w = request(h, "GET", "/api/snapshot", "", viewer, ""); w.Code != 401 {
		t.Fatal("password reset left an old session valid")
	}
	if w = request(h, "POST", "/api/session", `{"username":"viewer","password":"a replacement test passphrase"}`, nil, ""); w.Code != 200 {
		t.Fatal("replacement password does not work", w.Body.String())
	}
	if w = request(h, "PUT", "/api/users/admin/role", `{"role":"viewer"}`, admin, csrf); w.Code != 400 {
		t.Fatal("last administrator demoted")
	}
	if w = request(h, "DELETE", "/api/users/admin", `{}`, admin, csrf); w.Code != 400 {
		t.Fatal("current administrator deleted")
	}
	if w = request(h, "PUT", "/api/users/operator/projects", `{"projects":["../private"]}`, admin, csrf); w.Code != 400 {
		t.Fatal("invalid project grant accepted")
	}
}
func TestSignOutEverywhereAndIdleRefresh(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	first, csrf := signIn(t, h, "viewer")
	second, _ := signIn(t, h, "viewer")
	key := sha256.Sum256([]byte(first.Value))
	h.auth.mu.Lock()
	s := h.auth.sessions[key]
	s.LastActive = time.Now().Add(-10 * time.Minute)
	h.auth.sessions[key] = s
	h.auth.mu.Unlock()
	if w := request(h, "POST", "/api/session/refresh", `{}`, first, csrf); w.Code != 200 {
		t.Fatal(w.Code)
	}
	h.auth.mu.Lock()
	s = h.auth.sessions[key]
	h.auth.mu.Unlock()
	if time.Since(s.LastActive) > time.Minute {
		t.Fatal("human activity did not refresh idle timeout")
	}
	if w := request(h, "DELETE", "/api/account/sessions", `{}`, first, csrf); w.Code != 200 {
		t.Fatal(w.Code)
	}
	for _, cookie := range []*http.Cookie{first, second} {
		if w := request(h, "GET", "/api/snapshot", "", cookie, ""); w.Code != 401 {
			t.Fatal("a revoked browser session survived")
		}
	}
}

func TestAlertHistoryRetainsResolvedStateAndRespectsProjectScope(t *testing.T) {
	h, _ := newConsole(t, &fixtureOperator{}, true)
	if result := request(h, "GET", "/api/alerts", "", nil, ""); result.Code != 401 {
		t.Fatalf("anonymous history access: %d", result.Code)
	}
	viewer, _ := signIn(t, h, "viewer")
	result := request(h, "GET", "/api/alerts", "", viewer, "")
	if result.Code != 200 || strings.Contains(result.Body.String(), "private-alert") || strings.Contains(result.Body.String(), "global-alert") {
		t.Fatalf("scoped history: %d %s", result.Code, result.Body.String())
	}
	var alerts []models.Alert
	if err := json.Unmarshal(result.Body.Bytes(), &alerts); err != nil || len(alerts) != 1 || !alerts[0].Resolved || alerts[0].ResolvedAt == nil {
		t.Fatalf("resolved state missing: %v %+v", err, alerts)
	}
	admin, _ := signIn(t, h, "admin")
	result = request(h, "GET", "/api/alerts", "", admin, "")
	if result.Code != 200 || !strings.Contains(result.Body.String(), "private-alert") || !strings.Contains(result.Body.String(), "global-alert") {
		t.Fatalf("fleet history: %d %s", result.Code, result.Body.String())
	}
}
