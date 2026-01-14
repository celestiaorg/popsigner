package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Bidon15/popsigner/control-plane/cmd/popsigner-lite/internal/keystore"
)

// SetupRouter creates and configures the Gin router with all API routes.
func SetupRouter(ks *keystore.Keystore) *gin.Engine {
	// Create router
	router := gin.Default()

	// Add CORS middleware
	router.Use(corsMiddleware())

	// Health check endpoint (root)
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, HealthResponse{
			Status:  "ok",
			Version: "1.0.0",
		})
	})

	// API v1 routes
	v1 := router.Group("/v1")
	{
		// Keys endpoints
		keysHandler := NewKeysHandler(ks)
		v1.GET("/keys", keysHandler.ListKeys)
		v1.GET("/keys/:id", keysHandler.GetKey)
		v1.POST("/keys", keysHandler.CreateKey)
		v1.DELETE("/keys/:id", keysHandler.DeleteKey)

		// Signing endpoints
		signHandler := NewSignHandler(ks)
		v1.POST("/keys/:id/sign", signHandler.Sign)
		v1.POST("/sign/batch", signHandler.BatchSign)
	}

	return router
}

// corsMiddleware adds CORS headers for development.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
