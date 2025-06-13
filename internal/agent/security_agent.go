package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// LLMProvider interface for LLM interactions
type LLMProvider interface {
	CallLLM(ctx context.Context, prompt string, systemPrompt string) (string, error)
}

// SecurityAgent orchestrates the multi-step security analysis
type SecurityAgent struct {
	llm    LLMProvider
	config AgentConfig
}

// NewSecurityAgent creates a new security agent instance
func NewSecurityAgent(llm LLMProvider, config AgentConfig) *SecurityAgent {
	return &SecurityAgent{
		llm:    llm,
		config: config,
	}
}

// ProcessScan executes the complete 4-step agent workflow
func (a *SecurityAgent) ProcessScan(ctx context.Context, targetType, target, trivyJSON string) (*AgentResponse, error) {
	start := time.Now()
	requestID := uuid.New().String()

	log.Info().
		Str("request_id", requestID).
		Str("target", target).
		Str("target_type", targetType).
		Msg("Starting security agent processing")

	response := &AgentResponse{
		RequestID:   requestID,
		Timestamp:   start,
		TargetType:  targetType,
		Target:      target,
		StepResults: make(map[AgentStep]interface{}),
	}

	// Step 1: Analyze vulnerabilities
	analysis, err := a.analyzeVulnerabilities(ctx, trivyJSON)
	if err != nil {
		return nil, fmt.Errorf("analysis step failed: %w", err)
	}
	response.Analysis = *analysis
	response.StepResults[StepAnalyze] = analysis

	// Step 2: Prioritize vulnerabilities
	priorities, err := a.prioritizeVulnerabilities(ctx, analysis)
	if err != nil {
		return nil, fmt.Errorf("prioritization step failed: %w", err)
	}
	response.Priorities = priorities
	response.StepResults[StepPrioritize] = priorities

	// Step 3: Generate fixes
	fixes, err := a.generateFixes(ctx, analysis, priorities, targetType)
	if err != nil {
		return nil, fmt.Errorf("fix generation step failed: %w", err)
	}
	response.StepResults[StepGenerateFixes] = fixes

	// Step 4: Create remediation package
	remediationPackage, err := a.createRemediationPackage(ctx, analysis, fixes, targetType, target)
	if err != nil {
		return nil, fmt.Errorf("package creation step failed: %w", err)
	}
	response.RemediationPackage = *remediationPackage
	response.StepResults[StepCreatePackage] = remediationPackage

	response.ExecutionTime = time.Since(start)

	log.Info().
		Str("request_id", requestID).
		Dur("execution_time", response.ExecutionTime).
		Int("vulnerabilities_found", analysis.TotalVulnerabilities).
		Int("fixes_generated", len(fixes)).
		Msg("Security agent processing completed")

	return response, nil
}

// Step 1: Analyze vulnerabilities from Trivy JSON
func (a *SecurityAgent) analyzeVulnerabilities(ctx context.Context, trivyJSON string) (*SecurityAnalysis, error) {
	systemPrompt := `You are a security vulnerability analyzer. Your task is to parse Trivy scan results and provide structured analysis.

	Rules:
	1. Parse the JSON and extract vulnerability information
	2. Calculate risk scores based on CVSS, severity, and exploitability
	3. Provide clear categorization by severity
	4. Return ONLY valid JSON - no markdown, no explanations
	5. Use the exact schema provided`

	prompt := fmt.Sprintf(`
	Analyze this Trivy scan result and return a JSON response matching this exact schema:

	{
		"total_vulnerabilities": 0,
		"by_severity": {"CRITICAL": 0, "HIGH": 0, "MEDIUM": 0, "LOW": 0},
		"vulnerabilities": [
			{
				"id": "CVE-2023-1234",
				"title": "vulnerability title",
				"description": "brief description",
				"severity": "HIGH",
				"cvss": 7.5,
				"package": "package-name",
				"version": "1.0.0",
				"fixed_in": "1.0.1",
				"references": ["https://cve.mitre.org/..."]
			}
		],
		"risk_score": 85,
		"summary": "Brief analysis summary focusing on the most critical issues"
	}

	Trivy JSON Output:
	%s
	`, trivyJSON)

	response, err := a.llm.CallLLM(ctx, prompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var analysis SecurityAnalysis
	if err := json.Unmarshal([]byte(response), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis response: %w", err)
	}

	return &analysis, nil
}

// Step 2: Prioritize vulnerabilities based on risk factors
func (a *SecurityAgent) prioritizeVulnerabilities(ctx context.Context, analysis *SecurityAnalysis) ([]Priority, error) {
	systemPrompt := `You are a security risk prioritization expert. Your task is to prioritize vulnerabilities based on:
	1. CVSS score and severity
	2. Exploitability in the wild
	3. Business impact potential
	4. Availability of fixes
	
	Return ONLY valid JSON - no markdown, no explanations.`

	vulnData, _ := json.Marshal(analysis.Vulnerabilities)

	prompt := fmt.Sprintf(`
	Prioritize these vulnerabilities and return JSON matching this schema:

	[
		{
			"vulnerability_id": "CVE-2023-1234",
			"priority": 1,
			"reasoning": "Critical vulnerability with active exploits and easy fix available",
			"impact": "Remote code execution possible",
			"exploitability": "High - active exploits in the wild"
		}
	]

	Priority scale: 1 = Critical/Immediate, 2 = High/This Week, 3 = Medium/This Month, 4 = Low/Next Quarter, 5 = Informational

	Vulnerabilities to prioritize:
	%s
	`, string(vulnData))

	response, err := a.llm.CallLLM(ctx, prompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var priorities []Priority
	if err := json.Unmarshal([]byte(response), &priorities); err != nil {
		return nil, fmt.Errorf("failed to parse priorities response: %w", err)
	}

	return priorities, nil
}

// Step 3: Generate specific fixes for vulnerabilities
func (a *SecurityAgent) generateFixes(ctx context.Context, analysis *SecurityAnalysis, priorities []Priority, targetType string) ([]Fix, error) {
	systemPrompt := `You are a security remediation expert. Generate specific, actionable fixes for vulnerabilities.

	Rules:
	1. Provide exact commands, version numbers, and file changes
	2. Focus on the highest priority vulnerabilities first
	3. Consider the target type (Dockerfile, K8s manifest, etc.)
	4. Return ONLY valid JSON - no markdown, no explanations`

	// Only process high-priority vulnerabilities (priority <= threshold)
	highPriorityVulns := make([]Vulnerability, 0)
	priorityMap := make(map[string]int)

	for _, p := range priorities {
		priorityMap[p.VulnerabilityID] = p.Priority
	}

	for _, vuln := range analysis.Vulnerabilities {
		if priority, exists := priorityMap[vuln.ID]; exists && priority <= a.config.PriorityThreshold {
			highPriorityVulns = append(highPriorityVulns, vuln)
		}
	}

	vulnData, _ := json.Marshal(highPriorityVulns)
	priorityData, _ := json.Marshal(priorities)

	prompt := fmt.Sprintf(`
	Generate specific fixes for these vulnerabilities in a %s. Return JSON matching this schema:

	[
		{
			"vulnerability_id": "CVE-2023-1234",
			"type": "dependency_update",
			"description": "Update Node.js base image to fix vulnerability",
			"current_value": "FROM node:16",
			"recommended_value": "FROM node:18-alpine",
			"file_path": "Dockerfile",
			"line_number": 1,
			"command": "docker build -t updated-image .",
			"validation_steps": ["Run security scan", "Test application startup"]
		}
	]

	Fix types: "dependency_update", "config_change", "base_image_update", "package_removal"

	Target Type: %s
	Vulnerabilities: %s
	Priorities: %s
	`, targetType, targetType, string(vulnData), string(priorityData))

	response, err := a.llm.CallLLM(ctx, prompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var fixes []Fix
	if err := json.Unmarshal([]byte(response), &fixes); err != nil {
		return nil, fmt.Errorf("failed to parse fixes response: %w", err)
	}

	return fixes, nil
}

// Step 4: Create complete remediation package
func (a *SecurityAgent) createRemediationPackage(ctx context.Context, analysis *SecurityAnalysis, fixes []Fix, targetType, target string) (*RemediationPackage, error) {
	systemPrompt := `You are a DevOps automation expert. Create comprehensive remediation packages with proper commit messages, PR descriptions, and testing steps.

	Return ONLY valid JSON - no markdown, no explanations.`

	fixData, _ := json.Marshal(fixes)

	prompt := fmt.Sprintf(`
	Create a remediation package for these security fixes. Return JSON matching this schema:

	{
		"fixes": [...], // Use the provided fixes array as-is
		"commit_message": "fix: resolve 3 critical security vulnerabilities\n\n- Update Node.js base image (CVE-2023-1234)\n- Upgrade dependencies (CVE-2023-5678)",
		"pr_title": "Security fixes: resolve 3 critical vulnerabilities in %s",
		"pr_description": "## Security Fixes\n\nThis PR addresses 3 critical vulnerabilities:\n\n- **CVE-2023-1234**: Description and fix\n- **CVE-2023-5678**: Description and fix\n\n## Testing\n- [ ] Security scan passes\n- [ ] Application starts successfully",
		"testing_steps": ["Run updated security scan", "Verify application functionality", "Check for regression issues"]
	}

	Target: %s (%s)
	Risk Score: %d/100
	Fixes to package: %s
	`, target, target, targetType, analysis.RiskScore, string(fixData))

	response, err := a.llm.CallLLM(ctx, prompt, systemPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var pkg RemediationPackage
	if err := json.Unmarshal([]byte(response), &pkg); err != nil {
		return nil, fmt.Errorf("failed to parse package response: %w", err)
	}

	// Ensure fixes are included
	pkg.Fixes = fixes

	return &pkg, nil
}
