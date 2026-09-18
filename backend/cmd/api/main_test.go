package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	aegis "github.com/orcawhisperer/aegiscortex"
)

func TestRegisterRoutesDoesNotPanicAndServesStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := aegis.NewCortexEngineWithKey("")
	api, err := aegis.NewServerHandler(engine)
	if err != nil {
		t.Fatal(err)
	}
	router := gin.New()
	registerRoutes(router, api)

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/svc/api/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"status":"ok"`) || !strings.Contains(status.Body.String(), `"listen"`) {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}
	if status.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatalf("status must inherit security headers, XFO=%q", status.Header().Get("X-Frame-Options"))
	}

	head := httptest.NewRecorder()
	router.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/svc/api/healthz", nil))
	if head.Code != http.StatusOK {
		t.Fatalf("HEAD healthz = %d", head.Code)
	}

	state := httptest.NewRecorder()
	router.ServeHTTP(state, httptest.NewRequest(http.MethodGet, "/svc/api/state", nil))
	if state.Code != http.StatusOK || !strings.Contains(state.Body.String(), "rag_verified_fastpath") || !strings.Contains(state.Body.String(), `"listen"`) {
		t.Fatalf("state = %d %s", state.Code, state.Body.String())
	}

	put := httptest.NewRecorder()
	router.ServeHTTP(put, httptest.NewRequest(http.MethodPut, "/svc/api/status", nil))
	if put.Code != http.StatusMethodNotAllowed && put.Code != http.StatusNotFound {
		t.Fatalf("PUT status = %d", put.Code)
	}

	boot := httptest.NewRecorder()
	router.ServeHTTP(boot, httptest.NewRequest(http.MethodGet, "/svc/api/boot?case=sde_hallucinated_date", nil))
	if boot.Code != http.StatusOK || !strings.Contains(boot.Body.String(), "sde_hallucinated_date") {
		t.Fatalf("boot permalink case = %d %s", boot.Code, boot.Body.String())
	}

	bt := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/svc/api/backtest", strings.NewReader(`{"synthetic":4,"monthly_volume":10000000}`))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(bt, req)
	if bt.Code != http.StatusOK || !strings.Contains(bt.Body.String(), `"turns":4`) {
		t.Fatalf("backtest = %d %s", bt.Code, bt.Body.String())
	}
}
