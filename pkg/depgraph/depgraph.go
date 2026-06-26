package depgraph

import (
	"bytes"
	"encoding/json"
	"io"
	"os/exec"
	"strings"

	"pi/pkg/logger"
)

type PackageInfo struct {
	ImportPath string   `json:"ImportPath"`
	Name       string   `json:"Name"`
	Imports    []string `json:"Imports"`
}

func PrintDepGraph() {
	cmd := exec.Command("go", "list", "-json", "./...")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		logger.Log.Error("[오류] go list 실행 실패: %v\n", err)
		return
	}

	dec := json.NewDecoder(&stdout)
	var pkgs []PackageInfo

	for {
		var pkg PackageInfo
		if err := dec.Decode(&pkg); err == io.EOF {
			break
		} else if err != nil {
			logger.Log.Error("[오류] JSON 파싱 실패: %v\n", err)
			return
		}
		pkgs = append(pkgs, pkg)
	}

	logger.Log.Info("\n================= [의존성 그래프 (Import Graph)] =================\n")
	for _, pkg := range pkgs {
		if !strings.HasPrefix(pkg.ImportPath, "pi") {
			continue
		}

		logger.Log.Info(" 📦 %s\n", pkg.ImportPath)

		hasInternalDep := false
		for _, imp := range pkg.Imports {
			if strings.HasPrefix(imp, "pi") {
				logger.Log.Info("   └── 🔗 %s\n", imp)
				hasInternalDep = true
			}
		}

		if !hasInternalDep {
			logger.Log.Info("   └── (내부 의존성 없음)\n")
		}
		logger.Log.Info("\n")
	}
	logger.Log.Info("==================================================================\n")
}
