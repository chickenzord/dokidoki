package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chickenzord/dokidoki/internal/cluster"
	"github.com/chickenzord/dokidoki/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestAuthEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	password := "supersecret123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		StacksDir:    tempDir,
		ClusterToken: "cluster-tok-xyz",
		PasswordHash: string(hash),
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	handler := server.Routes()

	// 1. GET /api/v1/auth/me initially without cookie
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var meResp map[string]bool
	err = json.Unmarshal(w.Body.Bytes(), &meResp)
	require.NoError(t, err)
	assert.True(t, meResp["auth_required"])
	assert.False(t, meResp["authenticated"])

	// 2. Access protected endpoint before login -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 3. POST /api/v1/auth/login with wrong password -> 401
	loginBody, _ := json.Marshal(map[string]string{"password": "wrong"})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 4. POST /api/v1/auth/login with correct password -> 200 and session cookie
	loginBody, _ = json.Marshal(map[string]string{"password": password})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	cookies := w.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)
	assert.NotEmpty(t, sessionCookie.Value)
	assert.True(t, sessionCookie.HttpOnly)

	// 5. GET /api/v1/auth/me with session cookie -> authenticated: true
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	_ = json.Unmarshal(w.Body.Bytes(), &meResp)
	assert.True(t, meResp["authenticated"])

	// 6. Access protected endpoint with cookie -> 200
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 7. Change password
	changeBody, _ := json.Marshal(map[string]string{
		"current_password": password,
		"new_password":     "brandnewsecret456",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(changeBody))
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Dokidoki-Client", "web")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Verify new password is saved in auth.json
	authCfg, err := config.LoadAuthConfig(config.AuthFilePath(tempDir))
	require.NoError(t, err)
	err = bcrypt.CompareHashAndPassword([]byte(authCfg.AdminPasswordHash), []byte("brandnewsecret456"))
	assert.NoError(t, err)

	// 8. Logout
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	// Cookie cleared
	clearedCookies := w.Result().Cookies()
	var clearedSession *http.Cookie
	for _, c := range clearedCookies {
		if c.Name == SessionCookieName {
			clearedSession = c
			break
		}
	}
	require.NotNil(t, clearedSession)
	assert.True(t, clearedSession.MaxAge < 0 || clearedSession.Expires.Before(time.Now()))
}

func TestRemoteVsLoopbackAccess_NoPassword(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		Bind:      "0.0.0.0",
		Port:      8080,
		StacksDir: tempDir,
		// No PasswordHash
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	handler := server.Routes()

	// 1. Loopback request (127.0.0.1) -> allowed
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.RemoteAddr = "127.0.0.1:45678"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Loopback request (::1) -> allowed
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.RemoteAddr = "[::1]:45678"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 3. Remote request (10.0.0.5) without auth -> 401
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.RemoteAddr = "10.0.0.5:45678"
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Contains(t, w.Body.String(), "authentication required for remote access")
}

func TestClusterRouteTokenEnforcement(t *testing.T) {
	tempDir := t.TempDir()
	token := "cluster-secret-key"
	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		StacksDir:    tempDir,
		ClusterToken: token,
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	handler := server.Routes()

	// 1. POST /api/v1/nodes without token -> 401
	body := bytes.NewBufferString(`{"address":"http://192.168.1.50:8080"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/nodes", body)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// 2. DELETE /api/v1/nodes/{id} without token -> 401
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/nodes/peer-1", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestProxyHardening(t *testing.T) {
	// Setup mock remote peer
	var receivedClusterToken string
	peerMux := http.NewServeMux()
	peerMux.HandleFunc("/api/v1/stacks", func(w http.ResponseWriter, r *http.Request) {
		receivedClusterToken = r.Header.Get("X-Cluster-Token")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"remote-stack"}]`))
	})
	peerServer := httptest.NewServer(peerMux)
	defer peerServer.Close()

	remotePeer := cluster.Node{
		ID:        "peer-node",
		Name:      "remote",
		Addresses: []string{peerServer.URL},
	}
	mockCluster := &mockClusterService{nodes: []cluster.Node{remotePeer}}

	localToken := "local-node-cluster-token"
	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		ClusterToken: localToken,
	}

	server := NewServer(cfg, nil, nil, mockCluster, "")
	handler := server.Routes()

	// 1. Client attempts to pass a spoofed X-Cluster-Token -> stripped and replaced with local token
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes/peer-node/proxy/api/v1/stacks", nil)
	req.Header.Set("X-Cluster-Token", "malicious-injected-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, localToken, receivedClusterToken)

	// 2. Proxy recursion loop prevention: subPath contains /proxy -> 400
	req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/peer-node/proxy/api/v1/nodes/other/proxy/api/v1/stacks", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	// 3. SubPath must start with /api/v1/ -> 400
	req = httptest.NewRequest(http.MethodGet, "/api/v1/nodes/peer-node/proxy/etc/passwd", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestAuthLiveReload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		Bind:      "127.0.0.1",
		Port:      8080,
		StacksDir: tempDir,
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	handler := server.Routes()

	// 1. Initially no password -> auth not required
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	var meResp map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &meResp)
	require.NoError(t, err)
	assert.False(t, meResp["auth_required"])

	// 2. Simulate running dokidoki set-password by writing auth.json directly to disk
	newPw := "liveReloadSecret456!"
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPw), bcrypt.DefaultCost)
	require.NoError(t, err)

	authPath := config.AuthFilePath(tempDir)
	err = config.SaveAuthConfig(authPath, &config.AuthConfig{
		Version:           1,
		AdminPasswordHash: string(newHash),
		ClusterToken:      "live-reloaded-token",
	})
	require.NoError(t, err)

	// 3. GET /api/v1/auth/me now detects auth_required without server restart
	req = httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	err = json.Unmarshal(w.Body.Bytes(), &meResp)
	require.NoError(t, err)
	assert.True(t, meResp["auth_required"])
	assert.False(t, meResp["authenticated"])

	// 4. POST /api/v1/auth/login works immediately with new password
	loginBody := bytes.NewBufferString(`{"password":"liveReloadSecret456!"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", loginBody)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	cookies := w.Result().Cookies()
	require.NotEmpty(t, cookies)
	assert.Equal(t, SessionCookieName, cookies[0].Name)
}

func TestLoginRateLimiting(t *testing.T) {
	tempDir := t.TempDir()
	password := "correct-password-123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		StacksDir:    tempDir,
		PasswordHash: string(hash),
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	// Use shorter window for testing expiration (500ms > 5*65ms bcrypt delay)
	limiter := NewLoginRateLimiter(5, 500*time.Millisecond)
	server.WithLoginRateLimiter(limiter)
	handler := server.Routes()

	loginReq := func(ip, pw string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"password": pw})
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
		req.RemoteAddr = ip + ":12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		return w
	}

	testIP := "198.51.100.5"

	// 1. Five failed attempts
	for i := 1; i <= 5; i++ {
		w := loginReq(testIP, "wrong-pw")
		assert.Equal(t, http.StatusUnauthorized, w.Code, "attempt %d should be 401", i)
	}

	// 2. 6th attempt should be blocked with 429 Too Many Requests
	w := loginReq(testIP, "wrong-pw")
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	var errResp map[string]string
	err = json.Unmarshal(w.Body.Bytes(), &errResp)
	require.NoError(t, err)
	assert.Equal(t, "too many failed login attempts, please try again later", errResp["error"])

	// 3. Different IP is not blocked
	otherIP := "198.51.100.6"
	wOther := loginReq(otherIP, password)
	assert.Equal(t, http.StatusOK, wOther.Code)

	// 4. Wait for rate limit window to expire
	time.Sleep(550 * time.Millisecond)

	// 5. Attempt after expiry succeeds with correct password
	wExpired := loginReq(testIP, password)
	assert.Equal(t, http.StatusOK, wExpired.Code)

	// 6. Test successful login resets failure counter
	for i := 1; i <= 3; i++ {
		w := loginReq(testIP, "wrong-pw")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
	// Successful login resets
	wSuccess := loginReq(testIP, password)
	assert.Equal(t, http.StatusOK, wSuccess.Code)

	// Can fail another 4 times without 429
	for i := 1; i <= 4; i++ {
		w := loginReq(testIP, "wrong-pw")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	}
}

func TestCSRFProtectionMiddleware(t *testing.T) {
	tempDir := t.TempDir()
	password := "mypassword"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	cfg := &config.Config{
		Bind:         "127.0.0.1",
		Port:         8080,
		StacksDir:    tempDir,
		ClusterToken: "cluster-secret-token",
		PasswordHash: string(hash),
	}

	server := NewServer(cfg, &mockDocker{}, nil, nil, "")
	handler := server.Routes()

	// Login to obtain valid session cookie
	loginBody, _ := json.Marshal(map[string]string{"password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var sessionCookie *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == SessionCookieName {
			sessionCookie = c
			break
		}
	}
	require.NotNil(t, sessionCookie)

	// 1. GET request with cookie does not require CSRF header
	req = httptest.NewRequest(http.MethodGet, "/api/v1/stacks", nil)
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Mutating request (POST /api/v1/auth/change-password) with cookie and NO CSRF header -> 403 Forbidden
	changeBody, _ := json.Marshal(map[string]string{
		"current_password": password,
		"new_password":     "newpassword123",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(changeBody))
	req.AddCookie(sessionCookie)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusForbidden, w.Code)
	var errResp map[string]string
	_ = json.Unmarshal(w.Body.Bytes(), &errResp)
	assert.Equal(t, "missing required CSRF header", errResp["error"])

	// 3. Mutating request with X-Dokidoki-Client header -> 200 OK
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(changeBody))
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Dokidoki-Client", "web")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 4. Mutating request with X-Requested-With header -> 200 OK
	changeBody2, _ := json.Marshal(map[string]string{
		"current_password": "newpassword123",
		"new_password":     "newpassword456",
	})
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(changeBody2))
	req.AddCookie(sessionCookie)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 5. Cluster machine request with X-Cluster-Token bypasses CSRF check
	req = httptest.NewRequest(http.MethodPost, "/api/v1/cluster/leave", nil)
	req.Header.Set("X-Cluster-Token", "cluster-secret-token")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Response should not be 403 (either 200 or 400 bad request due to missing cluster manager / payload)
	assert.NotEqual(t, http.StatusForbidden, w.Code)
}

