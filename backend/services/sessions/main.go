package main

import "github.com/hieudang-lxp/ai-gateway/backend/internal/explore"

func main() { explore.Run("sessions", ":8791", "sessions-index-v1", true) }
