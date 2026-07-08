package main

import (
	"log"

	"github.com/Duliangheng2003/workflow-platform/internal/server"
)

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	cfg := server.DefaultConfig()
	if err := server.Run(cfg); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}