package aegiscortex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultBindHost = "127.0.0.1"
const defaultBindPort = "8090"

// defaultBindAddr is the last-resort local listen address when no PORT,
// AEGIS_ADDR, or AEGIS_PORT is set.
const defaultBindAddr = defaultBindHost + ":" + defaultBindPort

// SecurityHeadersMiddleware enforces web security headers and HTTP verb restrictions.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'; form-action 'self'",
		)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("X-DNS-Prefetch-Control", "off")

		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}

		if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		next.ServeHTTP(w, r)
	})
}

// NewServerHandler constructs the JSON API mux. The Next.js frontend owns HTML.
func NewServerHandler(engine *CortexEngine) (http.Handler, error) {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"has_api_key": engine.HasAPIKey(),
			"hosted":      HostedOnVercel(),
			"listen":      ResolvedListen(),
			"service":     "backend",
			"framework":   "go-gin",
		})
	})

	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"has_api_key": engine.HasAPIKey(),
			"hosted":      HostedOnVercel(),
			"listen":      ResolvedListen(),
			"presets":     engine.GetPresets(),
			"thresholds":  engine.GetThresholds(),
			"flywheel":    engine.GetFlywheel(),
			"sdk_version": "0.6.0",
		})
	})

	mux.HandleFunc("/api/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req EvaluationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
			return
		}
		evalCtx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
		defer cancel()
		res, err := engine.EvaluateRequest(evalCtx, req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "evaluation failed"})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	mux.HandleFunc("/api/thresholds", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var t PipelineThresholds
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid threshold JSON"})
			return
		}
		engine.SetThresholds(t)
		writeJSON(w, http.StatusOK, engine.GetThresholds())
	})

	mux.HandleFunc("/api/key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if HostedOnVercel() {
			writeJSON(w, http.StatusForbidden, map[string]string{
				"error": "On Vercel, set TYPESAFE_API_KEY in project environment variables. Browser key hold is local-only.",
			})
			return
		}
		var body struct {
			APIKey string `json:"api_key"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid key payload"})
			return
		}
		engine.SetAPIKey(body.APIKey)
		mode := "CALIBRATED_JEV_SIMULATION"
		if engine.HasAPIKey() {
			mode = "LIVE_TYPESAFE_API"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"has_api_key": engine.HasAPIKey(),
			"mode":        mode,
		})
	})

	return SecurityHeadersMiddleware(mux), nil
}

// HostedOnVercel reports the Vercel function/platform environment.
func HostedOnVercel() bool {
	return os.Getenv("VERCEL") == "1" ||
		os.Getenv("VERCEL_ENV") != "" ||
		os.Getenv("VERCEL_URL") != "" ||
		os.Getenv("VERCEL_REGION") != ""
}

func envPort(name string) (string, error) {
	port := strings.TrimSpace(os.Getenv(name))
	if port == "" {
		return "", nil
	}
	if strings.ContainsAny(port, ":/") {
		return "", fmt.Errorf("invalid %s %q", name, port)
	}
	return port, nil
}

func isLoopbackAddr(addr string) bool {
	return strings.HasPrefix(addr, "127.0.0.1:") || strings.HasPrefix(addr, "localhost:")
}

// ListenAddr is :PORT on a platform, otherwise loopback.
// 127.0.0.1:8090 is only the last-resort local default — not a baked-in
// production bind. Precedence: PORT (Vercel / any platform) → AEGIS_ADDR
// → AEGIS_PORT on 127.0.0.1 → 127.0.0.1:8090.
func ListenAddr() (string, error) {
	if port, err := envPort("PORT"); err != nil {
		return "", err
	} else if port != "" {
		// Gin's router.Run() / Vercel Fluid: all interfaces on PORT.
		return ":" + port, nil
	}
	if HostedOnVercel() {
		return ":3001", nil
	}

	if addr := strings.TrimSpace(os.Getenv("AEGIS_ADDR")); addr != "" {
		if !isLoopbackAddr(addr) {
			return "", fmt.Errorf("AEGIS_ADDR must bind to 127.0.0.1 or localhost, got %q", addr)
		}
		return addr, nil
	}

	if port, err := envPort("AEGIS_PORT"); err != nil {
		return "", err
	} else if port != "" {
		return defaultBindHost + ":" + port, nil
	}

	return defaultBindAddr, nil
}

// ResolvedListen is ListenAddr or empty when the policy rejects the env.
func ResolvedListen() string {
	addr, err := ListenAddr()
	if err != nil {
		return ""
	}
	return addr
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
