package app

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"pi/internal"
	"pi/pkg/logger"
	"pi/pkg/platform/windows"
)

func init() {
	_ = windows.EnableVTMode()
}

func printBanner() {
	logger.Log.Info(logger.ColorCyan + "██████╗ " + logger.ColorYellow + "██╗\n" +
		logger.ColorCyan + "██╔══██╗" + logger.ColorYellow + "██║\n" +
		logger.ColorCyan + "██████╔╝" + logger.ColorYellow + "██║\n" +
		logger.ColorCyan + "██╔═══╝ " + logger.ColorYellow + "██║\n" +
		logger.ColorCyan + "██║     " + logger.ColorYellow + "██║\n" +
		logger.ColorCyan + "╚═╝     " + logger.ColorYellow + "╚═╝\n" + logger.ColorReset)
}

func validatePythonInterpreter() error {
	_, err := internal.GetPythonVersion(internal.PythonCmd)
	if err != nil {
		logger.Log.Warn("파이썬 버전 확인 실패: %v\n", err)
	}
	// 성공 시 로그를 출력하지 않음
	return nil
}

func setupConfig(cfg *internal.DecryptedConfig) error {
	var modified bool

	if cfg.GoogleAPIKey == "" {
		logger.Log.Warn("설정 파일에서 API 키를 찾을 수 없음. GEMINI_API_KEY 환경변수 조회 중...\n")
		envKey := os.Getenv("GEMINI_API_KEY")
		if envKey != "" {
			internal.ApiKey = envKey
			cfg.GoogleAPIKey = envKey
			modified = true
		} else {
			logger.Log.Warn("API 키를 수동으로 입력하시오: ")
			var inputKey string
			fmt.Scanln(&inputKey)
			inputKey = strings.TrimSpace(inputKey)
			if inputKey == "" {
				return fmt.Errorf("유효한 API 키가 제공되지 않음")
			}
			internal.ApiKey = inputKey
			cfg.GoogleAPIKey = inputKey
			modified = true
		}
	} else {
		internal.ApiKey = cfg.GoogleAPIKey
	}

	if cfg.Model1 == "" {
		logger.Log.Warn("사용할 대상 모델을 입력하시오 (엔터 시 'gemini-2.5-flash-lite' 사용): ")
		var inputModel string
		fmt.Scanln(&inputModel)
		inputModel = strings.TrimSpace(inputModel)
		if inputModel == "" {
			inputModel = "gemini-2.5-flash-lite"
		}
		cfg.Model1 = inputModel
		modified = true
	}

	if modified {
		if err := internal.SaveConfig(cfg); err != nil {
			logger.Log.Warn("설정 저장 실패: %v\n", err)
		} else {
			logger.Log.Info("설정이 성공적으로 저장되었다.\n")
		}
	}

	return nil
}

func printHelp() {
	logger.Log.Info("\n==================================================\n")
	logger.Log.Info("                 [PI CLI 사용법 안내]\n")
	logger.Log.Info("==================================================\n")
	logger.Log.Info("PI CLI는 사용자의 요구사항에 맞게 파이썬 코드를 자동 생성 및\n")
	logger.Log.Info("실행하고, 결과를 분석해 주는 개발 보조 도구이다.\n")
	logger.Log.Info("※ 본 앱은 tea.exe에 의해 설정된 API 키와 대상 모델을 사용한다.\n")
	logger.Log.Info("\n")
	logger.Log.Info("[사용 가능한 내부 명령어]\n")
	logger.Log.Info("- help, h   : 본 도움말 및 사용법 안내 출력\n")
	logger.Log.Info("- stats     : 파이썬 코드 실행 성공/실패 통계 지표 확인\n")
	logger.Log.Info("- coverage  : 테스트 자동화율(Coverage) 현황 지표 진단\n")
	logger.Log.Info("- depgraph  : 패키지 간 의존성 그래프(Import Graph) 가시화\n")
	logger.Log.Info("- save      : 현재 대화 세션을 session.json 파일로 내보내기\n")
	logger.Log.Info("- load      : 기존 session.json 파일에서 대화 세션 불러오기\n")
	logger.Log.Info("- exit, q   : 프로그램 종료\n")
	logger.Log.Info("\n")
	logger.Log.Info("[기능 및 작동 흐름]\n")
	logger.Log.Info("1. 프롬프트 입력: 원하는 작업(예: '현재 폴더 파일 목록 보여줘')을 입력한다.\n")
	logger.Log.Info("2. 코드 생성: AI가 작업을 수행할 파이썬 코드를 생성한다.\n")
	logger.Log.Info("3. 자동 의존성 설치: 'requirements.txt' 존재 시 자동으로 패키지를 설치한다.\n")
	logger.Log.Info("4. 코드 실행: 생성된 코드가 타임아웃 제한 없이 실행된다.\n")
	logger.Log.Info("5. 결과 분석: 실행 결과에 대해 AI가 최종 분석 요약을 제공한다.\n")
	logger.Log.Info("6. 예외 복구: 실행 오류 발생 시 AI가 스스로 오류를 분석하고 재시도한다.\n")
	logger.Log.Info("==================================================\n")
}

func registerContextMenu() {
	exePath, err := os.Executable()
	if err != nil {
		return
	}

	exePath = strings.ReplaceAll(exePath, "\\", "\\\\")

	psScript := "$exePath = '" + exePath + "'\n" +
		"$key1 = 'HKCU:\\Software\\Classes\\Directory\\Background\\shell\\pi'\n" +
		"$key2 = 'HKCU:\\Software\\Classes\\Directory\\shell\\pi'\n\n" +
		"if (-not (Test-Path $key1)) { New-Item -Path $key1 -Force | Out-Null }\n" +
		"Set-ItemProperty -Path $key1 -Name '(Default)' -Value 'PI CLI 열기'\n" +
		"Set-ItemProperty -Path $key1 -Name 'Icon' -Value $exePath\n" +
		"if (-not (Test-Path \"$key1\\command\")) { New-Item -Path \"$key1\\command\" -Force | Out-Null }\n" +
		"Set-ItemProperty -Path \"$key1\\command\" -Name '(Default)' -Value \"`\"$exePath`\" `\"%V`\"\"\n\n" +
		"if (-not (Test-Path $key2)) { New-Item -Path $key2 -Force | Out-Null }\n" +
		"Set-ItemProperty -Path $key2 -Name '(Default)' -Value 'PI CLI 열기'\n" +
		"Set-ItemProperty -Path $key2 -Name 'Icon' -Value $exePath\n" +
		"if (-not (Test-Path \"$key2\\command\")) { New-Item -Path \"$key2\\command\" -Force | Out-Null }\n" +
		"Set-ItemProperty -Path \"$key2\\command\" -Name '(Default)' -Value \"`\"$exePath`\" `\"%V`\"\"\n"

	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", psScript)
	_ = cmd.Run()
}
