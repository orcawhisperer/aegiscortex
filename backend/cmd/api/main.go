package main

import (
	"log"
	"net/http"
	"os"
	"strings"
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
	router.GET("/svc/api", statusHandler(engine))
	router.GET("/svc/api/status", statusHandler(engine))
	router.Any("/svc/api/*path", func(c *gin.Context) {
		if c.Request.URL.Path == "/svc/api/status" || c.Request.URL.Path == "/svc/api" {
			statusHandler(engine)(c)
			return
		}
		req := c.Request.Clone(c.Request.Context())
		suffix := strings.TrimPrefix(c.Request.URL.Path, "/svc/api")
		if suffix == "" || suffix == "/" {
			req.URL.Path = "/healthz"
		} else {
			req.URL.Path = "/api" + suffix
		}
		api.ServeHTTP(c.Writer, req)
	})
}

func statusHandler(engine *aegis.CortexEngine) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service":     "backend",
			"framework":   "go-gin",
			"status":      "ok",
			"has_api_key": engine.HasAPIKey(),
			"hosted":      aegis.HostedOnVercel(),
			"timestamp":   time.Now().UTC().Format(time.RFC3339),
		})
	}
}
