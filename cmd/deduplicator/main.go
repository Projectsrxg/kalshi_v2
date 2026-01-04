package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	configPath := flag.String("config", "/etc/kalshi/deduplicator.yaml", "path to config file")
	flag.Parse()

	fmt.Printf("Starting deduplicator with config: %s\n", *configPath)

	// TODO: Initialize components
	// - Load configuration
	// - Connect to gatherer databases
	// - Connect to production RDS
	// - Start cursor-based sync polling
	// - Start deduplication workers
	// - Start S3 export (if configured)
	// - Start metrics server

	os.Exit(0)
}
