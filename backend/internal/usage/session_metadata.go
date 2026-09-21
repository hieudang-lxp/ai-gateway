package usage

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// Metadata only: never synthesize titles from prompt or response text.
func metadataText(value string, limit int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
	runes := []rune(value)
	if len(runes) > limit {
		runes = runes[:limit]
	}
	return string(runes)
}

type sessionName struct {
	ID        string    `json:"id"`
	Name      string    `json:"thread_name"`
	UpdatedAt time.Time `json:"updated_at"`
}

func readSessionNames(path string) (map[string]sessionName, time.Time, error) {
	names := map[string]sessionName{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return names, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, time.Time{}, err
	}
	err = lines(f, func(line []byte) error {
		var row sessionName
		if err := json.Unmarshal(line, &row); err != nil {
			return err
		}
		if row.ID != "" && !row.UpdatedAt.Before(names[row.ID].UpdatedAt) {
			row.Name = metadataText(row.Name, 512)
			names[row.ID] = row
		}
		return nil
	})
	return names, info.ModTime(), err
}
