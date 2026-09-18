package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	aegis "github.com/orcawhisperer/aegiscortex"
)

func main() {
	gin.SetMode(gin.ReleaseMode)

	addr, err := aegis.ListenAddr()
	if err != nil {
		log.Fatalf("security policy violation: %v", err)
	}

	engine := aegis.NewCortexEngineWithKey(os.Getenv("TYPESAFE_API_KEY"))
	api, err := aegis.NewServerHandler(engine)
	if err != nil {
		log.Fatalf("failed to initialize AegisCortex API: %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery())
	registerRoutes(router, engine, api)

	log.Printf("AegisCortex Gin backend on http://%s (sim unless TYPESAFE_API_KEY is set)", addr)
	if err := router.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func registerRoutes(router *gin.Engine, engine *aegis.CortexEngine, api http.Handler) {
	// Gin forbids mixing /svc/api/status with /svc/api/*path. Mount each
	// public path explicitly and proxy into the stdlib API mux.
	router.GET("/svc/api", statusHandler(engine))
	router.GET("/svc/api/status", statusHandler(engine))
	router.GET("/svc/api/healthz", proxy(api, "/healthz"))
	router.GET("/svc/api/state", proxy(api, "/api/state"))
	router.POST("/svc/api/evaluate", proxy(api, "/api/evaluate"))
	router.POST("/svc/api/thresholds", proxy(api, "/api/thresholds"))
	router.POST("/svc/api/key", proxy(api, "/api/key"))
}

func proxy(api http.Handler, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request.Clone(c.Request.Context())
		req.URL.Path = path
		api.ServeHTTP(c.Writer, req)
	}
}

func statusHandler(engine *aegis.CortexEngine) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":     "backend",
			"framework":   "go-gin",
			"status":      "ok",
			"has_api_key": engine.HasAPIKey(),
			"hosted":      aegis.HostedOnVercel(),
			"listen":      aegis.ResolvedListen(),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}
