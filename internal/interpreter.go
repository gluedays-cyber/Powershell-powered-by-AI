package internal

import (
	"os"
	"os/exec"
	"strings"
)

var PythonCmd = resolvePythonInterpreter()

func resolvePythonInterpreter() string {
	var pythonPath string

	if cfg, err := LoadConfig(); err == nil && cfg.PythonPath != "" {
		if _, err := os.Stat(cfg.PythonPath); err == nil {
			pythonPath = cfg.PythonPath
		}
	}

	if pythonPath == "" {
		if envPath := os.Getenv("PI_PYTHON"); envPath != "" {
			pythonPath = envPath
		} else if envPath := os.Getenv("PYTHON_INTERPRETER"); envPath != "" {
			pythonPath = envPath
		}
	}

	if pythonPath == "" {
		pythonPath = "python"
	}

	return pythonPath
}

func GetPythonVersion(pythonPath string) (string, error) {
	cmd := exec.Command(pythonPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
