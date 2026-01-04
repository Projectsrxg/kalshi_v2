package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "/etc/kalshi/gatherer.yaml", "path to config file")
	flag.Parse()

	fmt.Printf("Starting gatherer with config: %s\n", *configPath)

	// TODO: Initialize components
	// - Load configuration
	// - Connect to databases
	// - Start Market Registry
	// - Start Connection Manager
	// - Start Message Router
	// - Start Writers
	// - Start Snapshot Poller
	// - Start metrics server

	os.Exit(0)
}
