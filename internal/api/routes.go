package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func SetupRoutes() func(*gin.Engine) {
	return func(r *gin.Engine) {
		// Health check endpoint
		r.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"status":  "healthy",
				"service": "weekly-security-ai",
				"version": "1.0.0",
			})
		})

		// API v1 routes
		v1 := r.Group("/api/v1")
		{
			// Legacy scan endpoint (backward compatible)
			v1.POST("/scan", ScanHandler)

			// Agent-specific endpoints
			v1.POST("/agent/scan", AgentScanHandler)
			v1.GET("/agent/status", AgentStatusHandler)
		}

		// Root scan endpoint for backward compatibility
		r.POST("/scan", ScanHandler)
	}
}

// AgentScanHandler is a dedicated handler for agent-only operations
func AgentScanHandler(c *gin.Context) {
	var req ScanRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Force agent usage for this endpoint
	req.UseAgent = true

	// Call the main scan handler
	ScanHandler(c)
}

// AgentStatusHandler returns the current agent configuration and status
func AgentStatusHandler(c *gin.Context) {
	if securityAgent == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Security agent not initialized",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "ready",
		"agent_type": "multi_step_security_agent",
		"capabilities": []string{
			"vulnerability_analysis",
			"risk_prioritization",
			"fix_generation",
			"remediation_packaging",
		},
		"supported_targets": []string{
			"dockerfile",
			"kubernetes_manifest",
			"container_image",
		},
		"version": "1.0.0",
	})
}

// func SetupRoutes() func(*gin.Engine) {
// 	return func(r *gin.Engine) {
// 		r.POST("/scan", ScanHandler)
// 	}
// }
