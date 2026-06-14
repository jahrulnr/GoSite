package main

import (
	"fmt"
	"os"

	"github.com/jahrulnr/gosite/internal/app"
	"github.com/jahrulnr/gosite/internal/bootstrap"
	"github.com/jahrulnr/gosite/internal/config"
	"github.com/jahrulnr/gosite/internal/repository/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cfg := config.Load()
		if err := app.RunServe(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "gosite serve: %v\n", err)
			os.Exit(1)
		}
	case "init":
		if err := runInit(); err != nil {
			fmt.Fprintf(os.Stderr, "gosite init: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "gosite migrate: %v\n", err)
			os.Exit(1)
		}
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func runInit() error {
	cfg := config.Load()
	if err := bootstrap.Init(cfg); err != nil {
		return err
	}
	fmt.Println("gosite init: storage ready")
	return nil
}

func runMigrate() error {
	cfg := config.Load()
	db, err := sqlite.Open(cfg.Database)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := sqlite.Migrate(db, cfg.MigrationsDir); err != nil {
		return err
	}
	fmt.Println("gosite migrate: migrations applied")
	return nil
}

func printUsage() {
	fmt.Println(`gosite — BangunSite migration panel (Go backend)

Usage:
  gosite serve    Start HTTPS API server
  gosite init     First-boot storage initialization
  gosite migrate  Apply database migrations`)
}
