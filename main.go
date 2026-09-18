package main

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed templates/* static/*
var embeddedAssets embed.FS

const defaultBindAddr = "127.0.0.1:8090"

// PageTemplateData holds the server-side rendered data for templates/index.html.
type PageTemplateData struct {
	Title              string
	HasAPIKey          bool
	Presets            []ScenarioPreset
	Thresholds         PipelineThresholds
	InitialEval        EvaluationResponse
	InitialContextJSON string
	Flywheel           FlywheelTelemetryView
}

// SecurityHeadersMiddleware enforces mandatory web security headers and HTTP verb restrictions.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; script-src 'self' https://cdn.tailwindcss.com; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; font-src 'self' https://cdn.jsdelivr.net data:; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; object-src 'none'; base-uri 'self'",
		)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}

		if r.Method != http.MethodGet && r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB payload bound
		next.ServeHTTP(w, r)
	})
}

// NewServerHandler constructs the HTTP mux for AegisCortex.
func NewServerHandler(engine *CortexEngine) (http.Handler, error) {
	tmpl, err := template.ParseFS(embeddedAssets, "templates/index.html")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	// Static assets (/static/styles.css, /static/app.js)
	mux.Handle("/static/", http.FileServer(http.FS(embeddedAssets)))

	// GET / -> Render templates/index.html via Go html/template with initial evaluation telemetry
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		presets := engine.GetPresets()
		var defaultContext map[string]any
		for _, p := range presets {
			if p.ID == "rag_verified_fastpath" {
				defaultContext = p.Context
				break
			}
		}
		ctxBytes, _ := json.MarshalIndent(defaultContext, "", "  ")

		evalCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		initialEval, err := engine.EvaluateRequest(evalCtx, EvaluationRequest{
			ScenarioID: "rag_verified_fastpath",
			Context:    defaultContext,
		})
		if err != nil {
			http.Error(w, "failed initial evaluation", http.StatusInternalServerError)
			return
		}

		data := PageTemplateData{
			Title:              "AegisCortex — 100ms Speculative AI Control Plane & Arbitrage Studio",
			HasAPIKey:          engine.HasAPIKey(),
			Presets:            presets,
			Thresholds:         engine.GetThresholds(),
			InitialEval:        initialEval,
			InitialContextJSON: string(ctxBytes),
			Flywheel:           engine.GetFlywheel(),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "template render error", http.StatusInternalServerError)
		}
	})

	// GET /api/state
	mux.HandleFunc("/api/state", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"has_api_key": engine.HasAPIKey(),
			"presets":     engine.GetPresets(),
			"thresholds":  engine.GetThresholds(),
			"flywheel":    engine.GetFlywheel(),
		})
	})

	// POST /api/evaluate
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
		evalCtx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		res, err := engine.EvaluateRequest(evalCtx, req)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, res)
	})

	// POST /api/thresholds
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

	// POST /api/key
	mux.HandleFunc("/api/key", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func main() {
	addr := strings.TrimSpace(os.Getenv("AEGIS_ADDR"))
	if addr == "" {
		addr = defaultBindAddr
	}
	if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
		log.Fatalf("security policy violation: AEGIS_ADDR must bind to 127.0.0.1 or localhost, got %q", addr)
	}

	engine := NewCortexEngineWithKey(os.Getenv("TYPESAFE_API_KEY"))
	handler, err := NewServerHandler(engine)
	if err != nil {
		log.Fatalf("failed to initialize AegisCortex server: %v", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("AegisCortex Speculative 100ms AI Control Plane listening on http://%s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
