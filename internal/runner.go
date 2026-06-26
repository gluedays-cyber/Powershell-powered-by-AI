package internal

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

func runPythonFile(filename string) (string, string, error) {
	cmd := exec.Command(PythonCmd, filename)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONDONTWRITEBYTECODE=1", "PYTHONUNBUFFERED=1")
	// 임시 디렉터리 경로 대신, 명령을 실행한 원래의 작업 디렉터리 경로를 지정
	cmd.Dir = "."

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	if err != nil {
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return stdout.String(), stderr.String(), fmt.Errorf("파이썬 실행 파일을 찾을 수 없음: %s가 올바른 경로인지 확인 필요", PythonCmd)
		}
		if _, ok := err.(*exec.ExitError); !ok {
			return stdout.String(), stderr.String(), fmt.Errorf("파이썬 인터프리터 시작 실패: %w", err)
		}
	}
	return stdout.String(), stderr.String(), err
}

func validatePythonSyntax(filename string) error {
	cmd := exec.Command(PythonCmd, "-c", "import ast, sys; ast.parse(open(sys.argv[1], encoding='utf-8').read())", filename)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if execErr, ok := err.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return fmt.Errorf("파이썬 실행 파일을 찾을 수 없음: %s", PythonCmd)
		}
		if _, ok := err.(*exec.ExitError); !ok {
			return fmt.Errorf("문법 검사 시작 실패: %w", err)
		}
		return fmt.Errorf("SyntaxError: %s", strings.TrimSpace(stderr.String()))
	}
	return nil
}

func InstallRequirements() error {
	reqFile := "requirements.txt"
	if _, err := os.Stat(reqFile); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(reqFile)
	if err != nil {
		_ = os.Remove(reqFile)
		return nil
	}

	lines := strings.Split(string(data), "\n")
	hasValidReq := false
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			hasValidReq = true
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	if !hasValidReq {
		Log.Info("[의미 없는 requirements.txt 제거 진행]\n")
		_ = os.Remove(reqFile)
		return nil
	}

	defer os.Remove(reqFile)

	// 캐시 비교를 통해 동일 의존성 설치 스킵 처리
	reqContentNormalized := strings.Join(cleanedLines, "\n")
	hashFile := ".requirements_hash"
	cachedHash, hashErr := os.ReadFile(hashFile)
	
	// 해시 비교
	currentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(reqContentNormalized)))
	if hashErr == nil && string(cachedHash) == currentHash {
		// 이미 설치 완료된 동일한 의존성이므로 속도 단축을 위해 즉각 리턴
		return nil
	}

	Log.Info("[requirements.txt 감지됨. 의존성 패키지 자동 설치 중...]\n")
	cmd := exec.Command(PythonCmd, "-m", "pip", "install", "-r", reqFile)
	
	var outBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &outBuf)
	cmd.Stderr = io.MultiWriter(os.Stderr, &outBuf)

	runErr := cmd.Run()
	
	outputStr := outBuf.String()
	if strings.Contains(outputStr, "A new release of pip is available") || strings.Contains(outputStr, "--upgrade pip") {
		Log.Info("[pip 신규 버전 감지. 자동 업그레이드 진행 중...]\n")
		upgradeCmd := exec.Command(PythonCmd, "-m", "pip", "install", "--upgrade", "pip")
		upgradeCmd.Stdout = os.Stdout
		upgradeCmd.Stderr = os.Stderr
		if upgradeErr := upgradeCmd.Run(); upgradeErr == nil {
			restartApp()
		} else {
			Log.Warn("[pip 자동 업그레이드 실패] %v\n", upgradeErr)
		}
	}

	if runErr != nil {
		if execErr, ok := runErr.(*exec.Error); ok && execErr.Err == exec.ErrNotFound {
			return fmt.Errorf("파이썬 및 pip 실행 파일을 찾을 수 없음: %s", PythonCmd)
		}
		return fmt.Errorf("의존성 패키지 설치 실패: %w", runErr)
	}

	// 설치 성공 후 새로운 해시 기록 보관
	_ = os.WriteFile(hashFile, []byte(currentHash), 0644)
	return nil
}

func restartApp() {
	Log.Info("[애플리케이션 재실행 중...]\n")
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		Log.Error("재실행 실패: %v\n", err)
		return
	}
	os.Exit(0)
}
