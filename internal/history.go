package internal

import (
	"fmt"
	"os"
	"time"
)

func logExecutionHistory(pyCode string, stdout string, stderr string, execErr error) {
	f, err := os.OpenFile("log.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		Log.Warn("[로그 파일 열기 실패] %v\n", err)
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	var status string
	if execErr != nil {
		status = fmt.Sprintf("실패 (%v)", execErr)
	} else {
		status = "성공"
	}

	logContent := fmt.Sprintf(
		"=========================================\n"+
			"실행 시간: %s\n"+
			"실행 상태: %s\n"+
			"-----------------------------------------\n"+
			"[실행된 파이썬 코드]\n%s\n"+
			"-----------------------------------------\n"+
			"[표준 출력(Stdout)]\n%s\n"+
			"-----------------------------------------\n"+
			"[표준 에러(Stderr)]\n%s\n"+
			"=========================================\n\n",
		timestamp, status, pyCode, stdout, stderr,
	)

	if _, err := f.WriteString(logContent); err != nil {
		Log.Warn("[로그 파일 쓰기 실패] %v\n", err)
	}
}
