package internal

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"pi/pkg/fs"
	"pi/pkg/logger"
	"pi/pkg/metrics"
)

var Log logger.Logger = logger.Log
var globalPythonREPL *PythonREPL


func ExecuteGenericWorkflow(lang string, code string, input string, chatHistory []Content, aiResponse string, primaryModel string) (bool, string, []Content, error) {
	// 언어별 파일 확장자 및 빌드/실행 명령어 구성
	ext, compileCmd, runCmd, wingetId := getRuntimeConfig(lang)
	
	// 필요 도구 사전 설치 상태 점검 및 자동 설치 시도
	if err := ensureRuntimeInstalled(lang, wingetId); err != nil {
		Log.Warn("[런타임 자동 설치 시도 실패] %v\n", err)
	}

	filename, err := writeTempGenericFile(code, ext)
	if err != nil {
		return false, "", nil, err
	}
	defer os.Remove(filename)

	Log.Info("[%s 코드 실행 중...]\n", lang)
	if lang == "python" || lang == "py" {
		if err := InstallRequirements(); err != nil {
			Log.Warn("[의존성 설치 실패] %v\n", err)
		}
	}

	beforeState, _ := fs.GetDirState(".", "gen-cli-")

	var stdout, stderr string
	var execErr error

	if lang == "python" || lang == "py" {
		if globalPythonREPL == nil {
			var err error
			globalPythonREPL, err = NewPythonREPL()
			if err != nil {
				return false, "", nil, fmt.Errorf("파이썬 REPL 초기화 실패: %w", err)
			}
		}
		stdout, stderr, execErr = globalPythonREPL.Execute(code)
	} else if compileCmd != "" {
		// Java 등 컴파일이 필요한 경우 처리
		stdout, stderr, execErr = runCompileWorkflow(compileCmd, runCmd, filename)
	} else {
		stdout, stderr, execErr = runInterpreterWorkflow(runCmd, filename)
	}

	logExecutionHistory(code, stdout, stderr, execErr)

	afterState, _ := fs.GetDirState(".", "gen-cli-")
	diff := fs.CompareDirStates(beforeState, afterState)
	printFSChanges(diff)

	printExecutionOutputs(stdout, stderr, execErr)

	feedbackPrompt := fmt.Sprintf(
		"작성한 %s 코드 실행 결과는 다음과 같음.\n\n[표준 출력]\n%s\n\n[표준 에러]\n%s\n\n[실행 오류]\n%v\n\n결과를 요약하고 피드백할 것.",
		lang, stdout, stderr, execErr,
	)

	if execErr != nil {
		Log.Info("\n[안내] 실행 오류를 감지했다. 문제를 해결하기 위해 코드를 자동으로 재작성 중이니 잠시만 기다려 주기 바란다.\n")
	}

	feedbackResponse, err := callGeminiFeedback(primaryModel, chatHistory, aiResponse, feedbackPrompt)
	if err != nil {
		return false, "", nil, err
	}

	if execErr != nil {
		updatedHistory := buildRetryHistory(feedbackResponse, input, stderr, execErr, lang)
		metrics.RecordRun(false)
		metrics.PrintStats()
		return false, "", updatedHistory, nil
	}

	metrics.RecordRun(true)
	metrics.PrintStats()
	return true, feedbackResponse, nil, nil
}

func getRuntimeConfig(lang string) (ext string, compileCmd string, runCmd string, wingetId string) {
	switch lang {
	case "powershell", "pwsh", "ps1":
		return ".ps1", "", "powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File", ""
	case "python", "py":
		return ".py", "", PythonCmd, ""
	case "go":
		return ".go", "", "go run", "Golang.Go"
	case "javascript", "js":
		return ".js", "", "node", "OpenJS.NodeJS"
	case "java":
		return ".java", "javac", "java", "Oracle.JDK.21"
	case "cpp", "c++":
		return ".cpp", "g++", "", "msys2.msys2"
	default:
		// 디폴트는 powershell로 시도
		return ".ps1", "", "powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -File", ""
	}
}

func ensureRuntimeInstalled(lang string, wingetId string) error {
	if wingetId == "" {
		return nil
	}
	
	// 도구 실행 커맨드가 로컬 환경에 존재하는지 확인
	checkCmd := ""
	switch lang {
	case "go":
		checkCmd = "go"
	case "javascript", "js":
		checkCmd = "node"
	case "java":
		checkCmd = "java"
	case "cpp", "c++":
		checkCmd = "g++"
	}
	
	if checkCmd != "" {
		_, err := exec.LookPath(checkCmd)
		if err == nil {
			return nil // 이미 설치되어 있음
		}
	}

	Log.Warn("[경고] 시스템 내에 '%s' 실행 환경이 누락되어 있음. winget을 통해 자동 설치를 시도함...\n", lang)
	cmd := exec.Command("winget", "install", "--id", wingetId, "--silent", "--accept-source-agreements", "--accept-package-agreements")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

func writeTempGenericFile(code string, ext string) (string, error) {
	tmpFile, err := os.CreateTemp("", "gen-cli-*" + ext)
	if err != nil {
		return "", fmt.Errorf("임시 파일 생성 오류: %w", err)
	}
	filename := filepath.Clean(tmpFile.Name())

	if filename == "" || filename == "." || filename == "/" {
		tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("유효하지 않은 임시 파일 경로: %s", filename)
	}

	if err := tmpFile.Chmod(0644); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("파일 권한 설정 오류: %w", err)
	}

	if _, err := tmpFile.Write([]byte(code)); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("파일 저장 오류: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("파일 동기화 오류: %w", err)
	}
	tmpFile.Close()
	return filename, nil
}

func runInterpreterWorkflow(runCmd string, filename string) (string, string, error) {
	args := strings.Fields(runCmd)
	args = append(args, filename)
	
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = "."
	if args[0] == PythonCmd {
		cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8", "PYTHONDONTWRITEBYTECODE=1", "PYTHONUNBUFFERED=1")
	}

	var stdout, stderr bytes.Buffer
	// PowerShell 실행인 경우 실시간 화면 출력을 위해 Stdout/Stderr를 os.Stdout/os.Stderr와도 연결
	if strings.Contains(strings.ToLower(args[0]), "powershell") || strings.Contains(strings.ToLower(args[0]), "pwsh") {
		fmt.Printf("\n[PowerShell 실행 명령어]: %s %s\n", args[0], strings.Join(args[1:], " "))
		// Ensure UTF-8 output and Korean locale
		cmd.Env = append(os.Environ(), "LANG=ko_KR.UTF-8", "PYTHONIOENCODING=UTF-8")
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdout)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

func runCompileWorkflow(compileCmd string, runCmd string, filename string) (string, string, error) {
	// 컴파일
	compArgs := strings.Fields(compileCmd)
	compArgs = append(compArgs, filename)
	compCmd := exec.Command(compArgs[0], compArgs[1:]...)
	compCmd.Dir = "."
	
	var compStdout, compStderr bytes.Buffer
	compCmd.Stdout = &compStdout
	compCmd.Stderr = &compStderr
	
	if err := compCmd.Run(); err != nil {
		return compStdout.String(), compStderr.String(), fmt.Errorf("컴파일 실패: %w", err)
	}

	// 실행 파일 경로 도출 및 실행
	// 예: Java의 경우 클래스명 전달
	var runArgs []string
	if runCmd == "java" {
		// 임시 자바 파일 구조 분석하여 클래스 로딩
		classFile := strings.TrimSuffix(filepath.Base(filename), ".java")
		runArgs = []string{"java", "-cp", filepath.Dir(filename), classFile}
	} else {
		// C++ 등의 실행 바이너리
		exeFile := strings.TrimSuffix(filename, filepath.Ext(filename))
		if os.PathSeparator == '\\' {
			exeFile += ".exe"
		}
		runArgs = []string{exeFile}
		defer os.Remove(exeFile)
	}

	runExec := exec.Command(runArgs[0], runArgs[1:]...)
	runExec.Dir = "."
	
	var stdout, stderr bytes.Buffer
	runExec.Stdout = &stdout
	runExec.Stderr = &stderr

	err := runExec.Run()
	return stdout.String(), stderr.String(), err
}

func ExecutePowerShellWorkflow(psCode string, input string, chatHistory []Content, aiResponse string, primaryModel string) (bool, string, []Content, error) {
	return ExecuteGenericWorkflow("powershell", psCode, input, chatHistory, aiResponse, primaryModel)
}

func ExecutePythonWorkflow(pyCode string, input string, chatHistory []Content, aiResponse string, primaryModel string) (bool, string, []Content, error) {
	return ExecuteGenericWorkflow("python", pyCode, input, chatHistory, aiResponse, primaryModel)
}

func ExecuteGoWorkflow(goCode string, input string, chatHistory []Content, aiResponse string, primaryModel string) (bool, string, []Content, error) {
	return ExecuteGenericWorkflow("go", goCode, input, chatHistory, aiResponse, primaryModel)
}

func printFSChanges(diff fs.FsDiff) {
	var fsChanges strings.Builder
	if len(diff.Created) > 0 {
		fsChanges.WriteString(fmt.Sprintf("- 생성된 파일: %s\n", strings.Join(diff.Created, ", ")))
	}
	if len(diff.Modified) > 0 {
		fsChanges.WriteString(fmt.Sprintf("- 변경된 파일: %s\n", strings.Join(diff.Modified, ", ")))
	}
	if len(diff.Deleted) > 0 {
		fsChanges.WriteString(fmt.Sprintf("- 삭제된 파일: %s\n", strings.Join(diff.Deleted, ", ")))
	}
}

func printExecutionOutputs(stdout, stderr string, execErr error) {
	Log.Info("\n--- [코드 실행 출력] ---\n")
	if stdout != "" {
		Log.Info("[표준 출력]\n%s", stdout)
	}
	if stderr != "" {
		Log.Error("[표준 에러]\n%s", stderr)
	}
	if execErr != nil {
		Log.Error("[실행 오류] %v\n", execErr)
	}
	Log.Info("---------------------------\n")
}

func callGeminiFeedback(primaryModel string, chatHistory []Content, aiResponse string, feedbackPrompt string) (string, error) {
	tempHistory := append(chatHistory, Content{
		Role:  "model",
		Parts: []Part{{Text: aiResponse}},
	})
	tempHistory = append(tempHistory, Content{
		Role:  "user",
		Parts: []Part{{Text: feedbackPrompt}},
	})

	Log.Info("AI 실행 결과 분석 중...\n")
	var feedbackResponse string
	var callErr error
	for attempt := 1; attempt <= 3; attempt++ {
		feedbackResponse, callErr = CallGemini(primaryModel, tempHistory)
		if callErr == nil {
			break
		}
		Log.Warn("[API 오류 감지] %s 모델 호출 실패(피드백 시도 %d/3): %v. 재시도 진행...\n", primaryModel, attempt, callErr)
		if attempt < 3 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	if callErr != nil {
		return "", fmt.Errorf("API 최종 오류 (피드백): %w", callErr)
	}
	return feedbackResponse, nil
}

func buildRetryHistory(feedbackResponse, input, stderr string, execErr error, lang string) []Content {
	Log.Info("\nAI 피드백 >\n")
	Log.Info("%s\n", feedbackResponse)
	Log.Error("\n[오류 발생] %s 코드 실행 중 에러가 발생했다.\n", lang)

	errMsg := stderr
	if errMsg == "" {
		errMsg = execErr.Error()
	}
	errMsg = strings.TrimSpace(errMsg)

	retryPrompt := fmt.Sprintf(
		"사용자는 '%s'을 원하는데, %s 코드를 작성하여 실행한 결과, '%s'가 발생했으니, %s 코드를 다시 작성해서 시도해야 한다.",
		input, lang, errMsg, lang,
	)
	Log.Warn("\n[오류 감지] 아래 프롬프트로 재실행한다:\n%s\n\n", retryPrompt)
	return []Content{
		{
			Role:  "user",
			Parts: []Part{{Text: retryPrompt}},
		},
	}
}
