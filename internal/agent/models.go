package agent

import "time"

// AgentStep represents each step in the security analysis pipeline
type AgentStep string

const (
	StepAnalyze       AgentStep = "analyze"
	StepPrioritize    AgentStep = "prioritize"
	StepGenerateFixes AgentStep = "generate_fixes"
	StepCreatePackage AgentStep = "create_package"
)

// Vulnerability represents a parsed vulnerability from Trivy output
type Vulnerability struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    string   `json:"severity"`
	CVSS        float64  `json:"cvss,omitempty"`
	Package     string   `json:"package,omitempty"`
	Version     string   `json:"version,omitempty"`
	FixedIn     string   `json:"fixed_in,omitempty"`
	References  []string `json:"references,omitempty"`
}

// SecurityAnalysis represents the analysis step output
type SecurityAnalysis struct {
	TotalVulnerabilities int             `json:"total_vulnerabilities"`
	BySeverity           map[string]int  `json:"by_severity"`
	Vulnerabilities      []Vulnerability `json:"vulnerabilities"`
	RiskScore            int             `json:"risk_score"` // 1-100
	Summary              string          `json:"summary"`
}

// Priority represents the prioritization step output
type Priority struct {
	VulnerabilityID string `json:"vulnerability_id"`
	Priority        int    `json:"priority"` // 1-5 (1 = highest)
	Reasoning       string `json:"reasoning"`
	Impact          string `json:"impact"`
	Exploitability  string `json:"exploitability"`
}

// Fix represents a specific fix for a vulnerability
type Fix struct {
	VulnerabilityID  string   `json:"vulnerability_id"`
	Type             string   `json:"type"` // "dependency_update", "config_change", "base_image_update"
	Description      string   `json:"description"`
	CurrentValue     string   `json:"current_value,omitempty"`
	RecommendedValue string   `json:"recommended_value,omitempty"`
	FilePath         string   `json:"file_path,omitempty"`
	LineNumber       int      `json:"line_number,omitempty"`
	Command          string   `json:"command,omitempty"`
	ValidationSteps  []string `json:"validation_steps,omitempty"`
}

// RemediationPackage represents the final output with all fixes
type RemediationPackage struct {
	Fixes         []Fix    `json:"fixes"`
	CommitMessage string   `json:"commit_message"`
	PRTitle       string   `json:"pr_title"`
	PRDescription string   `json:"pr_description"`
	TestingSteps  []string `json:"testing_steps"`
}

// AgentResponse represents the complete agent output
type AgentResponse struct {
	RequestID          string                    `json:"request_id"`
	Timestamp          time.Time                 `json:"timestamp"`
	TargetType         string                    `json:"target_type"`
	Target             string                    `json:"target"`
	Analysis           SecurityAnalysis          `json:"analysis"`
	Priorities         []Priority                `json:"priorities"`
	RemediationPackage RemediationPackage        `json:"remediation_package"`
	ExecutionTime      time.Duration             `json:"execution_time"`
	StepResults        map[AgentStep]interface{} `json:"step_results,omitempty"`
}

// AgentConfig holds configuration for the security agent
type AgentConfig struct {
	MaxVulnerabilities int           `json:"max_vulnerabilities"`
	Timeout            time.Duration `json:"timeout"`
	EnableDebug        bool          `json:"enable_debug"`
	PriorityThreshold  int           `json:"priority_threshold"` // Only include fixes for priority <= this value
}
