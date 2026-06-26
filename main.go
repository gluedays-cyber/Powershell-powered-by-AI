package main

import (
	"log"

	"pi/cmd/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("애플리케이션 실행 오류: %v", err)
	}
}

