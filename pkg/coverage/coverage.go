package coverage

import (
	"os"
	"path/filepath"
	"strings"

	"pi/pkg/logger"
)

type FileCoverage struct {
	Path     string
	Tested   bool
	TestPath string
}

func PrintCoverageReport() {
	var files []FileCoverage
	root := "."

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == "node_modules" || name == "__pycache__" || name == ".idea" || name == ".vscode" || name == "winapi" {
				return filepath.SkipDir
			}
			return nil
		}

		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			testPath := strings.TrimSuffix(path, ".go") + "_test.go"
			tested := false
			if _, err := os.Stat(testPath); err == nil {
				tested = true
			}
			files = append(files, FileCoverage{
				Path:     filepath.ToSlash(path),
				Tested:   tested,
				TestPath: filepath.ToSlash(testPath),
			})
		}
		return nil
	})

	if err != nil {
		logger.Log.Error("[오류] 파일 구조 탐색 실패: %v\n", err)
		return
	}

	if len(files) == 0 {
		logger.Log.Info("\n프로젝트 내에 측정 대상 Go 소스 파일이 존재하지 않는다.\n")
		return
	}

	testedCount := 0
	logger.Log.Info("\n=================== [테스트 커버리지 현황] ===================\n")
	logger.Log.Info(" %-40s | %-10s\n", "소스 파일 경로", "테스트 여부")
	logger.Log.Info("--------------------------------------------------------------\n")
	for _, f := range files {
		status := "❌ 미완료"
		if f.Tested {
			status = "✅ 완료"
			testedCount++
		}
		logger.Log.Info(" %-40s | %-10s\n", f.Path, status)
	}

	coverageRate := (float64(testedCount) / float64(len(files))) * 100
	logger.Log.Info("--------------------------------------------------------------\n")
	logger.Log.Info(" 총 대상 소스 파일: %d 개\n", len(files))
	logger.Log.Info(" 테스트 작성 파일: %d 개\n", testedCount)
	logger.Log.Info(" 테스트 자동화율:   %.2f%%\n", coverageRate)
	logger.Log.Info("==============================================================\n")
}
