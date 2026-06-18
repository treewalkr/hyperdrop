package main

import (
	"fmt"
	"os"

	"github.com/treewalkr/hyperdrop/internal/cli"
	"github.com/treewalkr/hyperdrop/internal/server"
	"github.com/treewalkr/hyperdrop/internal/version"
)

func main() {
	cfg, err := cli.ParseArgs(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if cfg.ShowVersion {
		fmt.Println(version.String())
		return
	}

	if cfg.Token == "" {
		token, err := cli.GenerateToken()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		cfg.Token = token
	}

	if err := server.RunServer(cfg, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
