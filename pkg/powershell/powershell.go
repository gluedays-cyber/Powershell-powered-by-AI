package powershell

import (
	"bytes"
	"os/exec"
	"strings"
)

// PowerShell 구조체는 PowerShell 스크립트를 관리하고 실행하는 엔진 역할을 합니다.
type PowerShell struct {
	scripts []string
}

// Create 함수는 새로운 PowerShell 인스턴스를 생성하여 반환합니다.
func Create() *PowerShell {
	return &PowerShell{
		scripts: make([]string, 0),
	}
}

// AddScript는 실행할 PowerShell 스크립트 명령어를 추가합니다.
func (ps *PowerShell) AddScript(script string) {
	ps.scripts = append(ps.scripts, script)
}

// Invoke는 추가된 스크립트들을 PowerShell 엔진을 통해 실행하고 그 결과(표준 출력)를 줄 단위로 반환합니다.
func (ps *PowerShell) Invoke() ([]string, error) {
	if len(ps.scripts) == 0 {
		return []string{}, nil
	}

	// 추가된 스크립트들을 줄바꿈으로 연결하여 하나의 스크립트로 구성
	script := strings.Join(ps.scripts, "\n")

	// PowerShell 프로세스 생성 (-NoProfile, -NonInteractive 옵션으로 백그라운드 최적화)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", script)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 스크립트 실행
	err := cmd.Run()
	if err != nil {
		// 표준 에러 출력이 있는 경우 에러 메시지에 포함 (선택적)
		return nil, err
	}

	// 결과 문자열 처리
	outStr := strings.TrimSpace(stdout.String())
	if outStr == "" {
		return []string{}, nil
	}

	// 줄 단위로 분리하여 반환 (캐리지 리턴 제거)
	lines := strings.Split(outStr, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, "\r")
	}

	return lines, nil
}
