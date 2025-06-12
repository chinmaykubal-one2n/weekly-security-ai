package api

import (
	"net/http"
	"strings"
	"weeklysec/internal/llm"
	"weeklysec/internal/trivy"

	"github.com/gin-gonic/gin"
)

func ScanHandler(c *gin.Context) {
	var req struct {
		TargetType string `json:"target_type"` // "file" or "image"
		Target     string `json:"target"`      // path to file or image name
		Summarize  bool   `json:"summarize"`   // true if summary is needed
	}

	if err := c.ShouldBindJSON(&req); err != nil || req.TargetType == "" || req.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'target_type' and 'target' are required."})
		return
	}

	scanResult, err := trivy.RunScan(req.TargetType, req.Target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed", "details": err.Error()})
		return
	}

	// Handle summary
	if req.Summarize {
		summary, err := llm.Summarize(scanResult.RawOutput)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Summarization failed", "details": err.Error()})
			return
		}

		// Check if it's a CLI (curl/httpie) client
		ua := strings.ToLower(c.Request.UserAgent())
		isCLI := strings.Contains(ua, "curl") || strings.Contains(ua, "httpie")

		if isCLI {
			// return plain text summary
			c.String(http.StatusOK, summary)
			return
		}

		// else JSON response
		c.JSON(http.StatusOK, gin.H{
			"scan_results": scanResult,
			"summary":      summary,
		})
		return
	}

	// if Summarize == false
	c.JSON(http.StatusOK, gin.H{
		"scan_results": scanResult,
	})
}
