package parser

import (
	"bufio"
	"encoding/json"
	"os"

	"github.com/rs/zerolog/log"
)

type FileLogSource struct {
	Filename string
}

func (f *FileLogSource) ReadLogs() ([]LogEntry, error) {
	file, err := os.Open(f.Filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			log.Warn().Err(err).Msg("skipping invalid log line")
			continue
		}
		entries = append(entries, entry)
	}
	return entries, scanner.Err()
}
