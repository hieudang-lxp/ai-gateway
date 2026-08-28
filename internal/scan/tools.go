package scan

import (
	"encoding/json"
	"fmt"
)

// These mirror the serde JSON that lxp-scan emits (see ../../../lxp-scan
// features). Field tags match the exact JSON keys; the server maps them to
// proto. Keeping the decode here isolates all cgo/JSON handling from the gRPC
// layer.

type ImpactSite struct {
	Repo     string   `json:"repo"`
	File     string   `json:"file"`
	Line     uint32   `json:"line"`
	Source   string   `json:"source"`
	Refs     uint32   `json:"refs"`
	JsxUses  uint32   `json:"jsx_uses"`
	JsxProps []string `json:"jsx_props"`
	JsxLines []uint32 `json:"jsx_lines"`
}

type VersionCell struct {
	Version string `json:"version"`
	Source  string `json:"source"`
}

type DriftRow struct {
	Package  string                 `json:"package"`
	Versions map[string]VersionCell `json:"versions"`
	Level    string                 `json:"level"`
}

type DeclSite struct {
	Repo string `json:"repo"`
	File string `json:"file"`
	Line uint32 `json:"line"`
}

type DupeGroup struct {
	Name      string     `json:"name"`
	Sites     []DeclSite `json:"sites"`
	RepoCount uint32     `json:"repo_count"`
}

type CloneMember struct {
	Repo     string `json:"repo"`
	File     string `json:"file"`
	Line     uint32 `json:"line"`
	Name     string `json:"name"`
	Exported bool   `json:"exported"`
	Kind     string `json:"kind"`
	Sig      string `json:"sig"`
}

type CloneCluster struct {
	Members    []CloneMember `json:"members"`
	TokenCount uint32        `json:"token_count"`
	Sig        string        `json:"sig"`
	Literals   []string      `json:"literals"`
	Notes      []string      `json:"notes"`
}

type ClonesOutput struct {
	Clusters        []CloneCluster `json:"clusters"`
	NpmOnlyPackages []string       `json:"npm_only_packages"`
}

// PropCount decodes the ["name", count] tuple lxp-scan emits for prop_counts.
type PropCount struct {
	Name  string
	Count uint32
}

func (p *PropCount) UnmarshalJSON(b []byte) error {
	var t []json.RawMessage
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	if len(t) != 2 {
		return fmt.Errorf("prop_counts tuple has %d elements, want 2", len(t))
	}
	if err := json.Unmarshal(t[0], &p.Name); err != nil {
		return err
	}
	return json.Unmarshal(t[1], &p.Count)
}

type Definition struct {
	Repo    string `json:"repo"`
	File    string `json:"file"`
	Line    uint32 `json:"line"`
	Excerpt string `json:"excerpt"`
}

type UsageExcerpt struct {
	Repo     string   `json:"repo"`
	File     string   `json:"file"`
	Line     uint32   `json:"line"`
	JsxProps []string `json:"jsx_props"`
	Code     string   `json:"code"`
}

type SameNameGroup struct {
	Repo     string `json:"repo"`
	Sites    uint32 `json:"sites"`
	FromHint string `json:"from_hint"`
}

type ContextPack struct {
	Symbol     string          `json:"symbol"`
	TotalSites uint32          `json:"total_sites"`
	TotalFiles uint32          `json:"total_files"`
	TotalRepos uint32          `json:"total_repos"`
	PropCounts []PropCount     `json:"prop_counts"`
	Definition *Definition     `json:"definition"`
	Excerpts   []UsageExcerpt  `json:"excerpts"`
	SameName   []SameNameGroup `json:"same_name"`
}

// callJSON runs a tool in JSON mode and unmarshals its payload into v. A
// tool-level error (IsError) is surfaced as a Go error.
func callJSON(root, name string, args map[string]any, v any) error {
	res, err := Call(root, name, args, true)
	if err != nil {
		return err
	}
	if res.IsError {
		return fmt.Errorf("%s: %s", name, res.Text)
	}
	if err := json.Unmarshal([]byte(res.Text), v); err != nil {
		return fmt.Errorf("decode %s json: %w", name, err)
	}
	return nil
}

func Impact(root, symbol, from string) ([]ImpactSite, error) {
	args := map[string]any{"symbol": symbol}
	if from != "" {
		args["from"] = from
	}
	var out []ImpactSite
	return out, callJSON(root, "impact", args, &out)
}

func Drift(root string) ([]DriftRow, error) {
	var out []DriftRow
	return out, callJSON(root, "drift", map[string]any{}, &out)
}

func Dupes(root string) ([]DupeGroup, error) {
	var out []DupeGroup
	return out, callJSON(root, "dupes", map[string]any{}, &out)
}

func Clones(root, symbol string, minTokens uint32, sameFile bool) (ClonesOutput, error) {
	args := map[string]any{}
	if symbol != "" {
		args["symbol"] = symbol
	}
	if minTokens > 0 {
		args["min_tokens"] = minTokens
	}
	if sameFile {
		args["same_file"] = true
	}
	var out ClonesOutput
	return out, callJSON(root, "clones", args, &out)
}

func Context(root, symbol, from string, sites uint32) (ContextPack, error) {
	args := map[string]any{"symbol": symbol}
	if from != "" {
		args["from"] = from
	}
	if sites > 0 {
		args["sites"] = sites
	}
	var out ContextPack
	return out, callJSON(root, "context", args, &out)
}
