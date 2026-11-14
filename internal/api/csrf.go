package api

import (
	"crypto/subtle"
	"net/http"
)

// CSRFProtectionMiddleware returns a middleware that validates a custom header
// on mutating requests (POST, PUT, DELETE, PATCH) when authenticated via cookie session.
//
// Mechanism: Custom Header Check.
// Browsers cannot attach custom headers on cross-site form submissions or simple cross-origin requests.
// Requests authenticated via X-Cluster-Token (node-to-node machine requests) or loopback without
// session cookie can bypass this check.
func (s *Server) csrfProtectionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			// Mutating request. Check if bypass applies:
			// 1. Cluster token authentication bypass
			if s.cfg != nil && s.cfg.ClusterToken != "" {
				token := r.Header.Get("X-Cluster-Token")
				if token == "" {
					token = r.URL.Query().Get("token")
				}
				if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(s.cfg.ClusterToken)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}

			// 2. If authenticated via cookie session, require custom header
			if s.isSessionValid(r) {
				if r.Header.Get("X-Dokidoki-Client") == "" && r.Header.Get("X-Requested-With") == "" {
					writeError(w, http.StatusForbidden, "missing required CSRF header")
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}
