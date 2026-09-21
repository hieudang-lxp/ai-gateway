package main

import "github.com/hieudang-lxp/ai-gateway/backend/internal/explore"

func main() { explore.Run("insights", ":8792", "insights-index-v1", false) }
