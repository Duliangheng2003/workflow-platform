package main

import (
	"flag"
	"log"
	"os"

	"github.com/Duliangheng2003/workflow-platform/internal/config"
	"github.com/Duliangheng2003/workflow-platform/internal/server"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

	// Also check env var
	if envPath := os.Getenv("CONFIG_PATH"); envPath != "" {
		configPath = &envPath
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if err := server.Run(cfg); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}