package endpoint

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func finalizeTypeScriptCode(raw string) string {
	code := strings.TrimSpace(raw) + "\n"
	formatted, err := formatTypeScriptWithPrettier(code)
	if err != nil {
		return code
	}
	return formatted
}

func formatTypeScriptWithPrettier(code string) (string, error) {
	if prettierPath, err := exec.LookPath("prettier"); err == nil {
		if out, runErr := runTSFormatter(code, prettierPath, "--parser", "typescript"); runErr == nil {
			return out, nil
		}
	}

	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return "", fmt.Errorf("neither prettier nor npx is available")
	}
	return runTSFormatter(code, npxPath, "prettier", "--parser", "typescript")
}

func runTSFormatter(code string, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...)
	cmd.Stdin = strings.NewReader(code)

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("run %s %s failed: %w", command, strings.Join(args, " "), err)
	}

	result := out.String()
	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("formatter returned empty output")
	}
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return result, nil
}
