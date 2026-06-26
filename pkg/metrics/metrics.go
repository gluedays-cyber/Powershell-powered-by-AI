package metrics

import (
	"encoding/json"
	"os"

	"pi/pkg/logger"
)

type ExecutionStats struct {
	TotalRuns   int `json:"total_runs"`
	SuccessRuns int `json:"success_runs"`
	FailedRuns  int `json:"failed_runs"`
}

const statsFile = "metrics.json"

func LoadStats() ExecutionStats {
	var stats ExecutionStats
	data, err := os.ReadFile(statsFile)
	if err != nil {
		return stats
	}
	_ = json.Unmarshal(data, &stats)
	return stats
}

func SaveStats(stats ExecutionStats) {
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(statsFile, data, 0644)
}

func RecordRun(success bool) {
	stats := LoadStats()
	stats.TotalRuns++
	if success {
		stats.SuccessRuns++
	} else {
		stats.FailedRuns++
	}
	SaveStats(stats)
}

func PrintStats() {
	stats := LoadStats()
	var successRate float64
	if stats.TotalRuns > 0 {
		successRate = (float64(stats.SuccessRuns) / float64(stats.TotalRuns)) * 100
	}

	logger.Log.Info("\n================ [실행 지표 통계] ================\n")
	logger.Log.Info(" 총 실행 횟수:  %d 회\n", stats.TotalRuns)
	logger.Log.Info(" 성공 횟수:    %d 회\n", stats.SuccessRuns)
	logger.Log.Info(" 실패 횟수:    %d 회\n", stats.FailedRuns)
	logger.Log.Info(" 실행 성공률:   %.2f%%\n", successRate)
	logger.Log.Info("==================================================\n")
}
