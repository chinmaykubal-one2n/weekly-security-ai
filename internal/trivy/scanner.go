package trivy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

type ScanResult struct {
	RawOutput string
}

func RunScan(targetType, target string) (*ScanResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if targetType == "file" {
		cmd = exec.CommandContext(ctx, "trivy", "config", "--format", "json", target)
	} else if targetType == "image" {
		cmd = exec.CommandContext(ctx, "trivy", "image", "--format", "json", target)
	} else {
		return nil, fmt.Errorf("invalid target type: %s", targetType)
	}

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("failed to run trivy scan: %w\n%s", err, out.String())
	}

	return &ScanResult{
		RawOutput: out.String(),
	}, nil
}
