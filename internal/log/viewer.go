package log

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/andymarthin/pgtransfer/internal/utils"
)

// Today is the date format used for log file names and for the default day.
const dateLayout = "2006-01-02"

// LogPath returns the file that holds the entries for a date. An empty date
// means today.
func LogPath(date string) string {
	if date == "" {
		date = time.Now().Format(dateLayout)
	}
	return filepath.Join(utils.GetLogDir(), date+".log")
}

// Entries returns the log entries recorded on a date, oldest first. An empty
// date means today.
//
// A missing file is not an error: it means nothing was logged that day, which
// is the normal state on a machine that has not run a command yet.
func Entries(date string) ([]Entry, error) {
	if date != "" {
		if _, err := time.Parse(dateLayout, date); err != nil {
			return nil, fmt.Errorf("invalid date %q: use YYYY-MM-DD", date)
		}
	}

	file, err := os.Open(LogPath(date))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var entries []Entry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		// A malformed line is skipped rather than failing the whole view: the
		// log is an append-only convenience, and one truncated write from an
		// interrupted command must not hide the rest of the day.
		if err := json.Unmarshal(line, &e); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Timestamp < entries[j].Timestamp
	})
	return entries, nil
}

// AvailableDates returns the dates that have a log file, newest first.
func AvailableDates() []string {
	paths, err := filepath.Glob(filepath.Join(utils.GetLogDir(), "*.log"))
	if err != nil {
		return nil
	}

	dates := make([]string, 0, len(paths))
	for _, p := range paths {
		base := filepath.Base(p)
		date := base[:len(base)-len(".log")]
		if _, err := time.Parse(dateLayout, date); err != nil {
			continue
		}
		dates = append(dates, date)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dates)))
	return dates
}
