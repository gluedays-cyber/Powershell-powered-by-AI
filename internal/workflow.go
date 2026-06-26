package internal

import (
	"fmt"
	"os"
	"time"

	"pi/pkg/logger"
)

func ExecuteWorkflow(input string, primaryModel string, chatHistory []Content) ([]Content, string, bool, error) {
	// Initialize persistent PowerShell REPL for context extraction
	ps, err := NewPowerShellREPL()
	if err != nil {
		logger.Log.Warn("PowerShell REPL 초기화 실패: %v", err)
	}
	defer func() {
		if ps != nil {
			_ = ps.Close()
		}
	}()

	if cfg, err := LoadConfig(); err == nil {
		if cfg.GoogleAPIKey != "" {
			ApiKey = cfg.GoogleAPIKey
		}
		if cfg.Model1 != "" {
			primaryModel = cfg.Model1
		}
	}

	currentPrompt := input
	// Retrieve PowerShell session context and embed into prompt
	if ps != nil {
		if ctx, err := GetShellContext(ps); err == nil {
			currentPrompt = fmt.Sprintf("[Shell Context]\n%s\n\n%s", ctx, currentPrompt)
		} else {
			logger.Log.Warn("Shell context 수집 오류: %v", err)
		}
	}

	chatHistory = append(chatHistory, Content{
		Role:  "user",
		Parts: []Part{{Text: currentPrompt}},
	})

	for {
		logger.Log.Info("AI 응답 생성 중...\n")
		var aiResponse string
		var callErr error
		for attempt := 1; attempt <= 3; attempt++ {
			aiResponse, callErr = CallGeminiStream(primaryModel, chatHistory, os.Stdout)
			if callErr == nil {
				fmt.Println() // 스트리밍 출력 후 줄바꿈
				break
			}
			logger.Log.Warn("[API 오류 감지] %s 모델 호출 실패(시도 %d/3): %v. 재시도 진행...\n", primaryModel, attempt, callErr)
			if attempt < 3 {
				time.Sleep(500 * time.Millisecond)
			}
		}

		
		if callErr != nil {
			return chatHistory, "", false, fmt.Errorf("API 최종 호출 오류: %w", callErr)
		}

		lang, code := ExtractCodeBlock(aiResponse)

		if lang != "" && code != "" {
			success, feedbackResponse, updatedHistory, err := ExecuteGenericWorkflow(lang, code, input, chatHistory, aiResponse, primaryModel)
			if err != nil {
				return chatHistory, "", false, fmt.Errorf("실행 워크플로 오류: %w", err)
			}
			if !success {
				chatHistory = updatedHistory
				continue
			}

			chatHistory = append(chatHistory, Content{
				Role:  "model",
				Parts: []Part{{Text: aiResponse}},
			})
			chatHistory = append(chatHistory, Content{
				Role:  "user",
				Parts: []Part{{Text: "성공적으로 실행됨. 다음은 실행 결과 분석 보고서:\n" + feedbackResponse}},
			})
			return chatHistory, feedbackResponse, true, nil
		} else {
			chatHistory = append(chatHistory, Content{
				Role:  "model",
				Parts: []Part{{Text: aiResponse}},
			})
			return chatHistory, aiResponse, false, nil
		}
	}
}
