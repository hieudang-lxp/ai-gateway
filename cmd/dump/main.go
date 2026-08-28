// Command dump prints the canonical lxp-scan JSON for each tool (debug aid).
package main

import (
	"fmt"
	"os"

	"lxp-scan-svc/internal/scan"
)

func main() {
	root := os.Args[1]
	calls := []struct {
		name string
		args map[string]any
	}{
		{"impact", map[string]any{"symbol": "Button", "from": "fake-lib"}},
		{"drift", map[string]any{}},
		{"dupes", map[string]any{}},
		{"clones", map[string]any{"symbol": "validateEmail"}},
		{"context", map[string]any{"symbol": "Button", "sites": 2}},
	}
	for _, c := range calls {
		r, err := scan.Call(root, c.name, c.args, true)
		if err != nil {
			fmt.Printf("### %s ERROR: %v\n", c.name, err)
			continue
		}
		fmt.Printf("### %s (is_error=%v)\n%s\n\n", c.name, r.IsError, r.Text)
	}
}
