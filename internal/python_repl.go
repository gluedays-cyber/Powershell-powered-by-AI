package internal

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
)

type PythonREPL struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	stderr *bufio.Reader
	mu     sync.Mutex
}

func NewPythonREPL() (*PythonREPL, error) {
	// -q: 배너 숨김, -u: 버퍼링 해제, -i: 대화형 모드(즉각 실행)
	cmd := exec.Command(PythonCmd, "-qui")
	
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	
	return &PythonREPL{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
		stderr: bufio.NewReader(stderr),
	}, nil
}

func (repl *PythonREPL) Execute(code string) (string, string, error) {
	repl.mu.Lock()
	defer repl.mu.Unlock()

	endMarker := "##end_of_execution##"
	
	// 파이썬 문법에 맞게 들여쓰기를 조정한 뒤 try-except 로 감싸서 주입
	codeLines := strings.Split(code, "\n")
	var indentedCode []string
	for _, line := range codeLines {
		indentedCode = append(indentedCode, "    "+line)
	}
	
	payload := fmt.Sprintf(`
import sys
import traceback
try:
%s
except Exception:
    print(traceback.format_exc(), file=sys.stderr)

print('%s', file=sys.stdout)
print('%s', file=sys.stderr)
`, strings.Join(indentedCode, "\n"), endMarker, endMarker)

	// 파이프에 코드 기록
	if _, err := repl.stdin.Write([]byte(payload + "\n")); err != nil {
		return "", "", err
	}

	var stdoutBuf strings.Builder
	var stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for {
			line, err := repl.stdout.ReadString('\n')
			if err != nil {
				break
			}
			if strings.Contains(line, endMarker) {
				break
			}
			stdoutBuf.WriteString(line)
		}
	}()

	go func() {
		defer wg.Done()
		for {
			line, err := repl.stderr.ReadString('\n')
			if err != nil {
				break
			}
			if strings.Contains(line, endMarker) {
				break
			}
			// 대화형 프롬프트 제거
			cleanedLine := strings.ReplaceAll(line, ">>> ", "")
			cleanedLine = strings.ReplaceAll(cleanedLine, "... ", "")
			if strings.TrimSpace(cleanedLine) != "" {
				stderrBuf.WriteString(cleanedLine)
			}
		}
	}()

	wg.Wait()
	
	return stdoutBuf.String(), stderrBuf.String(), nil
}

func (repl *PythonREPL) Close() {
	repl.mu.Lock()
	defer repl.mu.Unlock()
	if repl.stdin != nil {
		repl.stdin.Close()
	}
	if repl.cmd != nil && repl.cmd.Process != nil {
		repl.cmd.Process.Kill()
		repl.cmd.Wait()
	}
}
