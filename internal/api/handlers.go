package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"
	"weeklysec/internal/agent"
	"weeklysec/internal/llm"
	"weeklysec/internal/trivy"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// ScanRequest represents the request structure for scanning
type ScanRequest struct {
	TargetType  string             `json:"target_type"`            // "file" or "image"
	Target      string             `json:"target"`                 // path to file or image name
	Summarize   bool               `json:"summarize"`              // true if summary is needed (legacy)
	UseAgent    bool               `json:"use_agent"`              // true to use the multi-step agent
	AgentConfig *agent.AgentConfig `json:"agent_config,omitempty"` // optional agent configuration
}

// ScanResponse represents different response formats
type ScanResponse struct {
	ScanResults *trivy.ScanResult    `json:"scan_results,omitempty"`
	Summary     string               `json:"summary,omitempty"`
	AgentResult *agent.AgentResponse `json:"agent_result,omitempty"`
}

// Initialize the agent (you might want to do this in main.go instead)
var securityAgent *agent.SecurityAgent

func init() {
	// Initialize the LLM client
	llmClient, err := llm.NewAgentClient()
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize LLM client")
	}

	// Default agent configuration
	config := agent.AgentConfig{
		MaxVulnerabilities: 50,
		Timeout:            5 * time.Minute,
		EnableDebug:        true,
		PriorityThreshold:  3, // Process priority 1-3 (Critical, High, Medium)
	}

	securityAgent = agent.NewSecurityAgent(llmClient, config)
}

// ScanHandler handles vulnerability scanning with optional AI agent processing
func ScanHandler(c *gin.Context) {
	var req ScanRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if req.TargetType == "" || req.Target == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request. 'target_type' and 'target' are required.",
		})
		return
	}

	// Run the Trivy scan
	scanResult, err := trivy.RunScan(req.TargetType, req.Target)
	if err != nil {
		log.Error().
			Err(err).
			Str("target", req.Target).
			Str("target_type", req.TargetType).
			Msg("Trivy scan failed")

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Scan failed",
			"details": err.Error(),
		})
		return
	}

	response := ScanResponse{
		ScanResults: scanResult,
	}

	// Check if agent processing is requested
	if req.UseAgent {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
		defer cancel()

		log.Info().
			Str("target", req.Target).
			Str("target_type", req.TargetType).
			Msg("Starting agent processing")

		agentResult, err := securityAgent.ProcessScan(ctx, req.TargetType, req.Target, scanResult.RawOutput)
		if err != nil {
			log.Error().
				Err(err).
				Str("target", req.Target).
				Msg("Agent processing failed")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":        "Agent processing failed",
				"details":      err.Error(),
				"scan_results": scanResult, // Still return scan results
			})
			return
		}

		response.AgentResult = agentResult

		log.Info().
			Str("request_id", agentResult.RequestID).
			Dur("execution_time", agentResult.ExecutionTime).
			Int("vulnerabilities", agentResult.Analysis.TotalVulnerabilities).
			Int("fixes", len(agentResult.RemediationPackage.Fixes)).
			Msg("Agent processing completed successfully")

	} else if req.Summarize {
		// Legacy summarization for backward compatibility
		summary, err := llm.Summarize(scanResult.RawOutput)
		if err != nil {
			log.Error().
				Err(err).
				Str("target", req.Target).
				Msg("Summarization failed")

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Summarization failed",
				"details": err.Error(),
			})
			return
		}
		response.Summary = summary
	}

	// Check if it's a CLI client for plain text response
	ua := strings.ToLower(c.Request.UserAgent())
	isCLI := strings.Contains(ua, "curl") || strings.Contains(ua, "httpie")

	if isCLI && req.UseAgent && response.AgentResult != nil {
		// Return formatted text for CLI users
		c.String(http.StatusOK, formatAgentResponseForCLI(response.AgentResult))
		return
	} else if isCLI && response.Summary != "" {
		// Legacy summary for CLI
		c.String(http.StatusOK, response.Summary)
		return
	}

	// Return JSON response
	c.JSON(http.StatusOK, response)
}

// formatAgentResponseForCLI formats the agent response for command-line display
func formatAgentResponseForCLI(result *agent.AgentResponse) string {
	var output strings.Builder

	output.WriteString("=== SECURITY AGENT ANALYSIS ===\n\n")

	// Executive Summary
	output.WriteString("EXECUTIVE SUMMARY:\n")
	output.WriteString(fmt.Sprintf("- Target: %s (%s)\n", result.Target, result.TargetType))
	output.WriteString(fmt.Sprintf("- Total Vulnerabilities: %d\n", result.Analysis.TotalVulnerabilities))
	output.WriteString(fmt.Sprintf("- Risk Score: %d/100\n", result.Analysis.RiskScore))
	output.WriteString(fmt.Sprintf("- Execution Time: %v\n\n", result.ExecutionTime))

	// Vulnerability Breakdown
	output.WriteString("VULNERABILITY BREAKDOWN:\n")
	for severity, count := range result.Analysis.BySeverity {
		if count > 0 {
			output.WriteString(fmt.Sprintf("- %s: %d\n", severity, count))
		}
	}
	output.WriteString("\n")

	// High Priority Fixes
	if len(result.RemediationPackage.Fixes) > 0 {
		output.WriteString("RECOMMENDED FIXES:\n")
		for i, fix := range result.RemediationPackage.Fixes {
			output.WriteString(fmt.Sprintf("%d. %s\n", i+1, fix.Description))
			if fix.Command != "" {
				output.WriteString(fmt.Sprintf("   Command: %s\n", fix.Command))
			}
			if fix.CurrentValue != "" && fix.RecommendedValue != "" {
				output.WriteString(fmt.Sprintf("   Change: %s -> %s\n", fix.CurrentValue, fix.RecommendedValue))
			}
			output.WriteString("\n")
		}
	}

	// PR Information
	if result.RemediationPackage.PRTitle != "" {
		output.WriteString("PULL REQUEST INFORMATION:\n")
		output.WriteString(fmt.Sprintf("Title: %s\n", result.RemediationPackage.PRTitle))
		output.WriteString(fmt.Sprintf("Commit: %s\n\n", result.RemediationPackage.CommitMessage))
	}

	// Testing Steps
	if len(result.RemediationPackage.TestingSteps) > 0 {
		output.WriteString("TESTING STEPS:\n")
		for i, step := range result.RemediationPackage.TestingSteps {
			output.WriteString(fmt.Sprintf("%d. %s\n", i+1, step))
		}
	}

	return output.String()
}

// func ScanHandler(c *gin.Context) {
// 	var req struct {
// 		TargetType string `json:"target_type"` // "file" or "image"
// 		Target     string `json:"target"`      // path to file or image name
// 		Summarize  bool   `json:"summarize"`   // true if summary is needed
// 	}

// 	if err := c.ShouldBindJSON(&req); err != nil || req.TargetType == "" || req.Target == "" {
// 		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request. 'target_type' and 'target' are required."})
// 		return
// 	}

// 	scanResult, err := trivy.RunScan(req.TargetType, req.Target)
// 	if err != nil {
// 		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed", "details": err.Error()})
// 		return
// 	}

// 	// Handle summary
// 	if req.Summarize {
// 		summary, err := llm.Summarize(scanResult.RawOutput)
// 		if err != nil {
// 			c.JSON(http.StatusInternalServerError, gin.H{"error": "Summarization failed", "details": err.Error()})
// 			return
// 		}

// 		// Check if it's a CLI (curl/httpie) client
// 		ua := strings.ToLower(c.Request.UserAgent())
// 		isCLI := strings.Contains(ua, "curl") || strings.Contains(ua, "httpie")

// 		if isCLI {
// 			// return plain text summary
// 			c.String(http.StatusOK, summary)
// 			return
// 		}

// 		// else JSON response
// 		c.JSON(http.StatusOK, gin.H{
// 			"scan_results": scanResult,
// 			"summary":      summary,
// 		})
// 		return
// 	}

// 	// if Summarize == false
// 	c.JSON(http.StatusOK, gin.H{
// 		"scan_results": scanResult,
// 	})
// }
