package internal

import (
    "bufio"
    "fmt"
    "io"
    "os"
    "os/exec"
    "strings"
)

// PowerShellREPL provides a persistent PowerShell session with bidirectional communication.
// It spawns a PowerShell process that reads commands from stdin and writes output to stdout.
// The Execute method sends a command and returns its output when a marker line is encountered.

type PowerShellREPL struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    reader *bufio.Reader
}

// NewPowerShellREPL starts a new PowerShell process in interactive mode.
func NewPowerShellREPL() (*PowerShellREPL, error) {
    // -NoLogo -NoProfile disables startup scripts and banner.
    // -ExecutionPolicy Bypass allows script execution without prompts.
    // -Command - tells PowerShell to read commands from stdin.
    cmd := exec.Command("powershell", "-NoLogo", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", "-")
    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to obtain stdin pipe: %w", err)
    }
    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, fmt.Errorf("failed to obtain stdout pipe: %w", err)
    }
    // Forward PowerShell stderr directly to the host process stderr for visibility.
    cmd.Stderr = os.Stderr
    if err := cmd.Start(); err != nil {
        return nil, fmt.Errorf("failed to start PowerShell: %w", err)
    }
    // Initialize PowerShell with UTF-8 output encoding and signal ready.
    initCmd := "$OutputEncoding = [System.Text.UTF8Encoding]::new(); Write-Host __REPL_READY__"
    if _, err := io.WriteString(stdin, initCmd+"\n"); err != nil {
        return nil, fmt.Errorf("failed to initialize PowerShell REPL: %w", err)
    }
    // Wait for ready marker
    for {
        line, err := bufio.NewReader(stdout).ReadString('\n')
        if err != nil {
            return nil, fmt.Errorf("failed to read PowerShell init output: %w", err)
        }
        if strings.Contains(line, "__REPL_READY__") {
            break
        }
    }

    return &PowerShellREPL{cmd: cmd, stdin: stdin, stdout: stdout, reader: bufio.NewReader(stdout)}, nil
}

// Execute runs a PowerShell command and returns its stdout output.
// It appends a unique marker to detect the end of the command output.
func (ps *PowerShellREPL) Execute(command string) (string, error) {
    const marker = "__END_OF_OUTPUT__"
    // Ensure the marker line is printed on its own.
    fullCmd := fmt.Sprintf("%s\nWrite-Host %s\n", command, marker)
    if _, err := io.WriteString(ps.stdin, fullCmd); err != nil {
        return "", fmt.Errorf("failed to write to PowerShell stdin: %w", err)
    }
    var output strings.Builder
    for {
        line, err := ps.reader.ReadString('\n')
        if err != nil {
            return "", fmt.Errorf("failed to read PowerShell output: %w", err)
        }
        trimmed := strings.TrimSpace(line)
        if trimmed == marker {
            break
        }
        output.WriteString(line)
    }
    return strings.TrimSpace(output.String()), nil
}

// Close terminates the PowerShell process.
func (ps *PowerShellREPL) Close() error {
    if err := ps.stdin.Close(); err != nil {
        return err
    }
    return ps.cmd.Wait()
}

// GetShellContext collects the current PowerShell session context:
//   - Current working directory
//   - Environment variables
//   - Last error message (if any)
// The result is a JSON string for easy consumption by the AI.
func GetShellContext(ps *PowerShellREPL) (string, error) {
    // PowerShell script builds an ordered hashtable and converts it to compact JSON.
    script := `$obj = [ordered]@{ Cwd = (Get-Location).Path; Env = $env:; LastError = $error[0].Exception.Message }; $obj | ConvertTo-Json -Compress`
    return ps.Execute(script)
}
