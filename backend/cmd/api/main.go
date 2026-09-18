package main

import (
	"log"
	"net/http"
	"os"

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
	registerRoutes(router, api)

	log.Printf("AegisCortex Gin backend on http://%s (sim unless TYPESAFE_API_KEY is set)", addr)
	if err := router.Run(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func registerRoutes(router *gin.Engine, api http.Handler) {
	// Gin forbids mixing /svc/api/status with /svc/api/*path. Mount each
	// public path explicitly and proxy into the stdlib API mux so every
	// route inherits SecurityHeadersMiddleware.
	router.HandleMethodNotAllowed = true
	router.GET("/svc/api", proxy(api, "/status"))
	router.GET("/svc/api/status", proxy(api, "/status"))
	router.HEAD("/svc/api", proxy(api, "/status"))
	router.HEAD("/svc/api/status", proxy(api, "/status"))
	router.GET("/svc/api/healthz", proxy(api, "/healthz"))
	router.HEAD("/svc/api/healthz", proxy(api, "/healthz"))
	router.GET("/svc/api/state", proxy(api, "/api/state"))
	router.HEAD("/svc/api/state", proxy(api, "/api/state"))
	router.GET("/svc/api/boot", proxy(api, "/api/boot"))
	router.HEAD("/svc/api/boot", proxy(api, "/api/boot"))
	router.POST("/svc/api/evaluate", proxy(api, "/api/evaluate"))
	router.POST("/svc/api/thresholds", proxy(api, "/api/thresholds"))
	router.POST("/svc/api/key", proxy(api, "/api/key"))
	router.POST("/svc/api/compile", proxy(api, "/api/compile"))
	router.POST("/svc/api/calibrate", proxy(api, "/api/calibrate"))
}

func proxy(api http.Handler, path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request.Clone(c.Request.Context())
		req.URL.Path = path
		api.ServeHTTP(c.Writer, req)
	}
}
