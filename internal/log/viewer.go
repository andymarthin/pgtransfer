package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

// ReadLogs prints a formatted view of logs for the given date.
// If date is empty, it defaults to today's date.
func ReadLogs(date string) error {
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	logDir := utils.GetLogDir()
	filePath := filepath.Join(logDir, fmt.Sprintf("%s.log", date))

	file, err := os.Open(filePath)
	if err != nil {
		utils.PrintWarning(nil, "No logs found for %s", date)
		return nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var entries []Entry

	for scanner.Scan() {
		var e Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
	}

	if len(entries) == 0 {
		utils.PrintInfo(nil, "No entries found for %s", date)
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})

	utils.PrintTitle(nil, fmt.Sprintf("📘 Logs for %s", date))
	fmt.Println()

	for _, e := range entries {
		status := strings.ToUpper(e.Status)
		switch status {
		case "SUCCESS":
			fmt.Printf("%s [%s] ✅ %s (%s)\n",
				utils.ColorTextGreen(e.Timestamp),
				e.Command,
				e.Message,
				e.Duration,
			)
		case "FAILURE":
			fmt.Printf("%s [%s] ❌ %s (%s)\n",
				utils.ColorTextRed(e.Timestamp),
				e.Command,
				e.Message,
				e.Duration,
			)
		case "INFO":
			fmt.Printf("%s [%s] ℹ️  %s\n",
				utils.ColorTextBlue(e.Timestamp),
				e.Command,
				e.Message,
			)
		default:
			fmt.Printf("%s [%s] %s\n",
				utils.ColorTextYellow(e.Timestamp),
				e.Command,
				e.Message,
			)
		}
	}
	fmt.Println()
	return nil
}
