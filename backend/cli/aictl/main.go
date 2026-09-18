package main

import (
	"fmt"
	"os"

	"github.com/hieudang-lxp/ai-gateway/backend/internal/aictl"
)

func main() {
	if err := aictl.NewCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "aictl:", err)
		os.Exit(1)
	}
}
