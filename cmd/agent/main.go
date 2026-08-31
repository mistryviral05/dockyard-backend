package main

import "github.com/mistryviral05/dockyard/internal/agentd"

func main() {
	// Load the agent configuration from environment variables
	_ = agentd.MustLoad()

	// Start the agent with the loaded configuration

}
