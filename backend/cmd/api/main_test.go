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
	registerRoutes(router, engine, api)

	status := httptest.NewRecorder()
	router.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/svc/api/status", nil))
	if status.Code != http.StatusOK || !strings.Contains(status.Body.String(), `"status":"ok"`) {
		t.Fatalf("status = %d %s", status.Code, status.Body.String())
	}

	state := httptest.NewRecorder()
	router.ServeHTTP(state, httptest.NewRequest(http.MethodGet, "/svc/api/state", nil))
	if state.Code != http.StatusOK || !strings.Contains(state.Body.String(), "rag_verified_fastpath") {
		t.Fatalf("state = %d %s", state.Code, state.Body.String())
	}
}
