package main

import (
	"fmt"
	"os"

	"github.com/gosuda/aira/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	_ = cfg
	fmt.Println("aira: ready")
}
