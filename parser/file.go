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

func (f *FileLogSource) ReadLogs(fn func(LogEntry) error) error {
	file, err := os.Open(f.Filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			log.Warn().Err(err).Msg("skipping invalid log line")
			continue
		}
		if err := fn(entry); err != nil {
			return err
		}
	}
	return scanner.Err()
}
