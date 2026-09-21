package usage

import (
	"crypto/sha256"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/store"
)

func ParseCursorCSV(r io.Reader, email string) ([]store.ExternalUsage, error) {
	reader := csv.NewReader(r)
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}
	columns := map[string]int{}
	for i, h := range header {
		columns[strings.TrimPrefix(strings.TrimSpace(h), "\ufeff")] = i
	}
	for _, key := range []string{"Date", "User", "Model", "Input (w/ Cache Write)", "Input (w/o Cache Write)", "Cache Read", "Output Tokens", "Total Tokens", "Cost"} {
		if _, ok := columns[key]; !ok {
			return nil, fmt.Errorf("missing CSV column %s", key)
		}
	}
	var out []store.ExternalUsage
	users := map[string]bool{}
	occurrences := map[string]int{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		get := func(k string) string {
			if i, ok := columns[k]; ok {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		user := strings.ToLower(get("User"))
		users[user] = true
		if email != "" && !strings.EqualFold(email, user) {
			continue
		}
		ts, err := time.Parse(time.RFC3339Nano, get("Date"))
		if err != nil {
			return nil, fmt.Errorf("invalid CSV date")
		}
		counts := make([]int64, 5)
		for i, k := range []string{"Input (w/o Cache Write)", "Output Tokens", "Cache Read", "Input (w/ Cache Write)", "Total Tokens"} {
			v := get(k)
			if v == "" || v == "-" {
				v = "0"
			}
			counts[i], err = strconv.ParseInt(strings.ReplaceAll(v, ",", ""), 10, 64)
			if err != nil || counts[i] < 0 {
				return nil, fmt.Errorf("invalid CSV token count in %s", k)
			}
		}
		if counts[0]+counts[1]+counts[2]+counts[3] != counts[4] {
			return nil, fmt.Errorf("CSV token total does not match breakdown")
		}
		var cost *float64
		kind := "reported"
		v := strings.TrimPrefix(get("Cost"), "$")
		if strings.EqualFold(v, "free") {
			zero := 0.0
			cost = &zero
		} else if v == "" || v == "-" || strings.EqualFold(v, "included") {
			kind = "unavailable"
		} else {
			n, err := strconv.ParseFloat(v, 64)
			if err != nil || n < 0 || math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("invalid CSV cost")
			}
			cost = &n
		}
		key := fmt.Sprintf("%s|%s|%s|%s|%v", ts.Format(time.RFC3339Nano), user, get("Model"), get("Kind"), counts)
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
		occurrences[hash]++
		out = append(out, store.ExternalUsage{Source: "cursor", ID: fmt.Sprintf("csv:%s:%d", hash, occurrences[hash]), TS: ts, Model: get("Model"), Usage: store.Usage{Input: counts[0], Output: counts[1], CacheRead: counts[2], CacheWrite: counts[3]}, CostUSD: cost, CostKind: kind})
	}
	if email == "" && len(users) > 1 {
		return nil, fmt.Errorf("set Cursor account email before importing a multi-user export")
	}
	return out, nil
}
