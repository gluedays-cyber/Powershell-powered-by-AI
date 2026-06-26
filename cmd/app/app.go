package app

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"path/filepath"
	"bufio"

	"github.com/c-bata/go-prompt"
	"pi/internal"
	"pi/pkg/coverage"
	"pi/pkg/depgraph"
	"pi/pkg/logger"
	"pi/pkg/metrics"
)

// ---------------------------------------------------------
// 1. 메인 실행 및 초기화
// ---------------------------------------------------------
func Run() error {
	defer internal.Cleanup()

	// 글자색만 흰색으로 지정 (배경색 지정 안 함)
	fmt.Print("\033[37m")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		internal.Cleanup()
		os.Exit(0)
	}()

	// printBanner()
	if err := validatePythonInterpreter(); err != nil {
		return err
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		logger.Log.Warn("설정 로드 실패. 기본 구성을 활용해 지속함: %v\n", err)
		cfg = &internal.DecryptedConfig{}
	}

	if err := setupConfig(cfg); err != nil {
		return err
	}

	primaryModel := cfg.Model1
	if primaryModel == "" {
		primaryModel = "gemini-2.5-flash-lite"
	}

	// logger.Log.Info("[사용 모델]: %s\n", primaryModel)

	// 1. 컨텍스트 메뉴 자동 등록
	registerContextMenu()

	// 2. 우클릭 컨텍스트 메뉴 등을 통해 폴더 경로가 인자로 전달된 경우
	if len(os.Args) == 2 {
		if info, err := os.Stat(os.Args[1]); err == nil && info.IsDir() {
			_ = os.Chdir(os.Args[1])
		} 
	}

	// 3. 기본 시작 폴더를 홈 디렉터리로 고정 (인자가 없을 때)
	if len(os.Args) == 1 {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
	}

	runREPLLoop(primaryModel)
	return nil
}

// ---------------------------------------------------------
// 2. REPL(Read-Eval-Print Loop) 루프 처리
// ---------------------------------------------------------

func getHistoryFilePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".pi_history"
	}
	return filepath.Join(home, ".pi_history")
}

func loadHistory() []string {
	file, err := os.Open(getHistoryFilePath())
	if err != nil {
		return []string{}
	}
	defer file.Close()

	var history []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text != "" {
			history = append(history, text)
		}
	}
	if err := scanner.Err(); err != nil {
		return []string{}
	}
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	return history
}

func appendHistory(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	history := loadHistory()
	
	if len(history) > 0 && history[len(history)-1] == cmd {
		return
	}
	
	history = append(history, cmd)
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	
	file, err := os.Create(getHistoryFilePath())
	if err != nil {
		return
	}
	defer file.Close()
	
	for _, h := range history {
		fmt.Fprintln(file, h)
	}
}

func runREPLLoop(primaryModel string) {
	fmt.Println("Powered by Python & AI")
	var activeHistory []internal.Content

	executor := func(input string) {
		input = strings.TrimSpace(input)
		if input != "" {
			appendHistory(input)
		}
		var exit bool
		activeHistory, exit = handleREPLCommand(input, activeHistory, primaryModel)
		if exit {
			internal.Cleanup()
			os.Exit(0)
		}
	}

	completer := func(d prompt.Document) []prompt.Suggest {
		var s []prompt.Suggest
		w := d.GetWordBeforeCursor()
		text := d.TextBeforeCursor()

		if strings.TrimSpace(text) == "" {
			return []prompt.Suggest{}
		}

		if strings.HasPrefix(text, "cd ") {
			dir := "."
			basePrefix := w

			lastSlash := strings.LastIndexAny(w, "/\\")
			if lastSlash >= 0 {
				dir = w[:lastSlash+1]
				basePrefix = w[lastSlash+1:]
			}

			readDir := dir
			if strings.HasPrefix(readDir, "~") {
				if home, err := os.UserHomeDir(); err == nil {
					// Expand ~ only at the beginning
					readDir = home + readDir[1:]
				}
			}

			entries, err := os.ReadDir(readDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() {
						if strings.HasPrefix(strings.ToLower(entry.Name()), strings.ToLower(basePrefix)) {
							var suggestText string
							if dir == "." {
								suggestText = entry.Name()
							} else {
								suggestText = dir + entry.Name()
							}
							s = append(s, prompt.Suggest{Text: suggestText})
						}
					}
				}
				sort.Slice(s, func(i, j int) bool {
					return s[i].Text < s[j].Text
				})
			}
			return s
		}

		for _, cmd := range []string{"exit", "q", "stats", "coverage", "depgraph", "help", "h", "save", "load"} {
			if strings.HasPrefix(cmd, strings.ToLower(w)) {
				s = append(s, prompt.Suggest{Text: cmd})
			}
		}
		return s
	}

	p := prompt.New(
		executor,
		completer,
		prompt.OptionLivePrefix(func() (string, bool) {
			cwd, _ := os.Getwd()
			title := "PyAIShell"
			fmt.Printf("\033]0;%s\007", title)
			return fmt.Sprintf("PI %s> ", cwd), true
		}),
		prompt.OptionPrefixTextColor(prompt.White),
		prompt.OptionInputTextColor(prompt.White),
		prompt.OptionSuggestionTextColor(prompt.White),
		prompt.OptionDescriptionTextColor(prompt.White),
		prompt.OptionSelectedSuggestionTextColor(prompt.Black),
		prompt.OptionSelectedDescriptionTextColor(prompt.Black),
		prompt.OptionScrollbarThumbColor(prompt.White),
		prompt.OptionCompletionOnDown(),
		prompt.OptionHistory(loadHistory()),
	)
	p.Run()
}

// ---------------------------------------------------------
// 3. 명령어 및 워크플로우 실행 로직
// ---------------------------------------------------------
func handleREPLCommand(input string, activeHistory []internal.Content, primaryModel string) ([]internal.Content, bool) {
	if input == "exit" || input == "q" {
		return activeHistory, true
	}
	if input == "stats" {
		metrics.PrintStats()
		return activeHistory, false
	}
	if input == "coverage" {
		coverage.PrintCoverageReport()
		return activeHistory, false
	}
	if input == "depgraph" {
		depgraph.PrintDepGraph()
		return activeHistory, false
	}
	if input == "help" || input == "h" {
		printHelp()
		return activeHistory, false
	}
	if input == "save" {
		if err := internal.SaveSession(activeHistory); err != nil {
			logger.Log.Error("세션 저장 실패: %v\n", err)
		} else {
			logger.Log.Success("대화 세션 저장 완료 (session.json)\n")
		}
		return activeHistory, false
	}
	if input == "load" {
		history, err := internal.LoadSession()
		if err != nil {
			logger.Log.Error("세션 불러오기 실패: %v\n", err)
		} else {
			activeHistory = history
			logger.Log.Success("대화 세션 불러오기 완료 (%d개 메세지 복구됨)\n", len(activeHistory))
		}
		return activeHistory, false
	}
	if input == "" {
		return activeHistory, false
	}

	// 드라이브 이동 처리 (예: D: 또는 d:)
	if len(input) == 2 && input[1] == ':' {
		if err := os.Chdir(input); err != nil {
			logger.Log.Error("디렉터리 이동 실패: %v\n", err)
		}
		return activeHistory, false
	}

	// cd 명령어 처리 (예: cd 폴더명)
	if strings.HasPrefix(input, "cd ") {
		target := strings.TrimSpace(input[3:])
		if strings.HasPrefix(target, "~") {
			if home, err := os.UserHomeDir(); err == nil {
				target = home + target[1:]
			}
		}
		if err := os.Chdir(target); err != nil {
			logger.Log.Error("디렉터리 이동 실패: %v\n", err)
		}
		return activeHistory, false
	}
	if input == "cd" || input == "cd ~" {
		if home, err := os.UserHomeDir(); err == nil {
			_ = os.Chdir(home)
		}
		return activeHistory, false
	}

	fields := strings.Fields(input)
	if len(fields) > 0 {
		cmdName := fields[0]
		checkCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", fmt.Sprintf("if (Get-Command '%s' -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }", strings.ReplaceAll(cmdName, "'", "''")))
		if err := checkCmd.Run(); err == nil {
			psCmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", input)
			psCmd.Stdout = os.Stdout
			psCmd.Stderr = os.Stderr
			psCmd.Stdin = os.Stdin
			if err := psCmd.Run(); err != nil {
				logger.Log.Error("명령어 실행 오류: %v\n", err)
			}
			return activeHistory, false
		}
	}

	activeHistory = executeWorkflow(input, primaryModel, activeHistory)
	return activeHistory, false
}

func executeWorkflow(input string, primaryModel string, chatHistory []internal.Content) []internal.Content {
	updatedHistory, feedback, isCodeWorkflow, err := internal.ExecuteWorkflow(input, primaryModel, chatHistory)
	if err != nil {
		logger.Log.Error("%v. 복구 가능한 에러로 취급하여 메인 대기 모드로 돌아감.\n", err)
		return updatedHistory
	}

	if isCodeWorkflow {
		logger.Log.Success("\n==============================================\n")
		logger.Log.Success("성공적으로 작업을 수행했다.\n")
		logger.Log.Success("수행된 작업 요약:\n%s\n\n", feedback)
		logger.Log.Success("==============================================\n")
	} else {
		logger.Log.Info("%s\n", feedback)
	}

	return updatedHistory
}
