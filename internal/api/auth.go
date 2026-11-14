package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chickenzord/dokidoki/internal/config"
	"golang.org/x/crypto/bcrypt"
)

const (
	SessionCookieName = "dokidoki_session"
	SessionDuration   = 24 * time.Hour
)

// SessionManager manages signing and verifying HMAC-SHA256 session cookies.
type SessionManager struct {
	mu      sync.RWMutex
	signKey []byte
}

// NewSessionManager creates a SessionManager. If seedSecret is provided, it derives a signing key;
// otherwise it generates a secure random 32-byte key.
func NewSessionManager(seedSecret string) *SessionManager {
	var key []byte
	if seedSecret != "" {
		h := sha256.Sum256([]byte(seedSecret + ":dokidoki-session-v1"))
		key = h[:]
	} else {
		key = make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic(fmt.Sprintf("failed to generate random session key: %v", err))
		}
	}
	return &SessionManager{signKey: key}
}

// UpdateSecret updates the signing key derived from the secret.
func (sm *SessionManager) UpdateSecret(seedSecret string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if seedSecret != "" {
		h := sha256.Sum256([]byte(seedSecret + ":dokidoki-session-v1"))
		sm.signKey = h[:]
	}
}

// CreateSession generates a signed token string: base64(expiry):base64(signature)
func (sm *SessionManager) CreateSession(expiry time.Time) string {
	sm.mu.RLock()
	key := sm.signKey
	sm.mu.RUnlock()

	expStr := strconv.FormatInt(expiry.Unix(), 10)
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(expStr))
	sig := mac.Sum(nil)

	return fmt.Sprintf("%s.%s",
		base64.RawURLEncoding.EncodeToString([]byte(expStr)),
		base64.RawURLEncoding.EncodeToString(sig),
	)
}

// VerifySession verifies the signed session token and checks if it's expired.
func (sm *SessionManager) VerifySession(token string) bool {
	sm.mu.RLock()
	key := sm.signKey
	sm.mu.RUnlock()

	parts := strings.Split(token, ".")
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

	// Verify HMAC
	mac := hmac.New(sha256.New, key)
	mac.Write(expBytes)
	expectedSig := mac.Sum(nil)
	if subtle.ConstantTimeCompare(sig, expectedSig) != 1 {
		return false
	}

	// Verify expiry
	expUnix, err := strconv.ParseInt(string(expBytes), 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expUnix {
		return false
	}

	return true
}

func (s *Server) setSessionCookie(w http.ResponseWriter, expiry time.Time) {
	if s.sessionMgr == nil {
		return
	}
	token := s.sessionMgr.CreateSession(expiry)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiry,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) isSessionValid(r *http.Request) bool {
	if s.sessionMgr == nil {
		return false
	}
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return s.sessionMgr.VerifySession(cookie.Value)
}

// handleAuthMe handles GET /api/v1/auth/me (Public)
// Returns {"authenticated": bool, "auth_required": bool}
func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	authRequired := s.cfg != nil && s.cfg.PasswordHash != ""
	authenticated := s.isSessionValid(r)

	writeJSON(w, http.StatusOK, map[string]bool{
		"authenticated": authenticated,
		"auth_required": authRequired,
	})
}

// handleAuthLogin handles POST /api/v1/auth/login (Public)
// Accepts {"password": "..."}
func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if s.cfg == nil || s.cfg.PasswordHash == "" {
		// If no password configured, login is allowed immediately or no-op
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "no password configured"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid password")
		return
	}

	expiry := time.Now().Add(SessionDuration)
	s.setSessionCookie(w, expiry)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleAuthLogout handles POST /api/v1/auth/logout (Clears cookie)
func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleAuthChangePassword handles POST /api/v1/auth/change-password (Protected)
// Accepts {"current_password": "...", "new_password": "..."}
func (s *Server) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.NewPassword) == "" {
		writeError(w, http.StatusBadRequest, "new password cannot be empty")
		return
	}

	// Verify current password if one is already configured
	if s.cfg != nil && s.cfg.PasswordHash != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(s.cfg.PasswordHash), []byte(req.CurrentPassword)); err != nil {
			writeError(w, http.StatusUnauthorized, "invalid current password")
			return
		}
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	// Read existing auth.json or create new
	authPath := config.AuthFilePath(s.stacksDir)
	authCfg, err := config.LoadAuthConfig(authPath)
	if err != nil {
		if errors.Is(err, config.ErrInsecurePermissions) {
			writeError(w, http.StatusInternalServerError, "auth file has insecure permissions")
			return
		}
		// File might not exist
		authCfg = &config.AuthConfig{Version: 1}
	}

	authCfg.AdminPasswordHash = string(newHash)
	if s.cfg != nil && authCfg.ClusterToken == "" {
		authCfg.ClusterToken = s.cfg.ClusterToken
	}

	if err := config.SaveAuthConfig(authPath, authCfg); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to save auth file: %v", err))
		return
	}

	// Update server state
	if s.cfg != nil {
		s.cfg.PasswordHash = string(newHash)
	}

	// Refresh session cookie for current user
	expiry := time.Now().Add(SessionDuration)
	s.setSessionCookie(w, expiry)

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
