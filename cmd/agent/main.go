package main

import (
	"log"

	"github.com/mistryviral05/dockyard/internal/agentd"
)

func main() {
	cfg := agentd.MustLoad()
	logger := agentd.NewLogger(cfg)

	if err := agentd.StartAgent(cfg, logger); err != nil {
		log.Fatalf("agent: %s", err.Error())
	}
}
