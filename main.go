package main

import (
	"context"
	"embed"
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

//go:embed templates/* static/*
var embeddedAssets embed.FS

const defaultBindAddr = "127.0.0.1:8090"

// PageTemplateData is the server-rendered shell plus a JSON boot payload.
type PageTemplateData struct {
	Title              string
	HasAPIKey          bool
	Presets            []ScenarioPreset
	Thresholds         PipelineThresholds
	InitialEval        EvaluationResponse
	InitialContextJSON string
	Flywheel           FlywheelTelemetryView
	BootJSON           template.JS
}

type bootPayload struct {
	HasAPIKey  bool                  `json:"has_api_key"`
	Presets    []ScenarioPreset      `json:"presets"`
	Thresholds PipelineThresholds    `json:"thresholds"`
	Eval       EvaluationResponse    `json:"eval"`
	Flywheel   FlywheelTelemetryView `json:"flywheel"`
}

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

// NewServerHandler constructs the HTTP mux for AegisCortex.
func NewServerHandler(engine *CortexEngine) (http.Handler, error) {
	tmpl, err := template.ParseFS(embeddedAssets, "templates/index.html")
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(embeddedAssets)))

	mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, embeddedAssets, "static/favicon.svg")
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"has_api_key": engine.HasAPIKey(),
		})
	})

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

		evalCtx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
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
			Title:              "AegisCortex",
			HasAPIKey:          engine.HasAPIKey(),
			Presets:            presets,
			Thresholds:         engine.GetThresholds(),
			InitialEval:        initialEval,
			InitialContextJSON: string(ctxBytes),
			Flywheel:           engine.GetFlywheel(),
		}
		data.BootJSON = marshalBoot(bootPayload{
			HasAPIKey:  data.HasAPIKey,
			Presets:    data.Presets,
			Thresholds: data.Thresholds,
			Eval:       data.InitialEval,
			Flywheel:   data.Flywheel,
		})

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			http.Error(w, "template render error", http.StatusInternalServerError)
		}
	})

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

func marshalBoot(v any) template.JS {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	s := strings.ReplaceAll(string(b), "<", `\u003c`)
	s = strings.ReplaceAll(s, "\u2028", `\u2028`)
	s = strings.ReplaceAll(s, "\u2029", `\u2029`)
	return template.JS(s)
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
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("AegisCortex listening on http://%s (sim unless TYPESAFE_API_KEY is set)", addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	case sig := <-sigCh:
		log.Printf("received %s, shutting down", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown error: %v", err)
		}
	}
}
