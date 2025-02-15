package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/onemorebsmith/kaspastratum/src/kaspastratum"
	"gopkg.in/yaml.v2"
)

func main() {
	// Get the current working directory
	pwd, _ := os.Getwd()
	fullPath := path.Join(pwd, "config.yaml")

	// Load configuration file
	log.Printf("Loading config @ `%s`", fullPath)
	rawCfg, err := os.ReadFile(fullPath)
	if err != nil {
		log.Fatalf("Config file not found: %s", err)
	}

	// Parse YAML configuration
	var cfg kaspastratum.BridgeConfig
	if err := yaml.Unmarshal(rawCfg, &cfg); err != nil {
		log.Fatalf("Failed parsing config file: %s", err)
	}

	// Define command-line flags with default values from config
	flag.StringVar(&cfg.StratumPort, "stratum", cfg.StratumPort, "Stratum port to listen on (default `:5555`)")
	flag.BoolVar(&cfg.PrintStats, "stats", cfg.PrintStats, "Show periodic stats in console (default `true`)")
	flag.StringVar(&cfg.RPCServer, "slyvex", cfg.RPCServer, "Slyvex node address (default `localhost:28110`)")
	flag.DurationVar(&cfg.BlockWaitTime, "blockwait", cfg.BlockWaitTime, "Time to wait before requesting a new block (default `3s`)")
	flag.UintVar(&cfg.MinShareDiff, "mindiff", cfg.MinShareDiff, "Minimum share difficulty (default `4096`)")
	flag.BoolVar(&cfg.ClampPow2, "pow2clamp", cfg.ClampPow2, "Clamp difficulty to powers of 2 (default `true`)")
	flag.BoolVar(&cfg.VarDiff, "vardiff", cfg.VarDiff, "Enable variable difficulty (default `true`)")
	flag.UintVar(&cfg.SharesPerMin, "sharespermin", cfg.SharesPerMin, "Target shares per minute for vardiff (default `20`)")
	flag.BoolVar(&cfg.VarDiffStats, "vardiffstats", cfg.VarDiffStats, "Log vardiff stats every 10s (default `false`)")
	flag.UintVar(&cfg.ExtranonceSize, "extranonce", cfg.ExtranonceSize, "Extranonce size in bytes (default `0`)")
	flag.StringVar(&cfg.PromPort, "prom", cfg.PromPort, "Prometheus metrics address (default `:2112`)")
	flag.BoolVar(&cfg.UseLogFile, "log", cfg.UseLogFile, "Log to file instead of console (default `true`)")
	flag.StringVar(&cfg.HealthCheckPort, "hcp", cfg.HealthCheckPort, "Health check port (`/readyz` endpoint)")
	flag.Parse()

	// Print configuration summary
	log.Println("----------------------------------")
	log.Println("Initializing Slyvex Stratum Bridge")
	log.Printf("\tSlyvex Node:      %s", cfg.RPCServer)
	log.Printf("\tStratum Port:     %s", cfg.StratumPort)
	log.Printf("\tPrometheus:       %s", cfg.PromPort)
	log.Printf("\tPrint Stats:      %t", cfg.PrintStats)
	log.Printf("\tLog to File:      %t", cfg.UseLogFile)
	log.Printf("\tMin Share Diff:   %d", cfg.MinShareDiff)
	log.Printf("\tPower-of-2 Clamp: %t", cfg.ClampPow2)
	log.Printf("\tVarDiff Enabled:  %t", cfg.VarDiff)
	log.Printf("\tShares per Min:   %d", cfg.SharesPerMin)
	log.Printf("\tVarDiff Stats:    %t", cfg.VarDiffStats)
	log.Printf("\tBlock Wait Time:  %s", cfg.BlockWaitTime)
	log.Printf("\tExtranonce Size:  %d", cfg.ExtranonceSize)
	log.Printf("\tHealth Check:     %s", cfg.HealthCheckPort)
	log.Println("----------------------------------")

	// Start the bridge server
	if err := kaspastratum.ListenAndServe(cfg); err != nil {
		log.Fatalf("Bridge server error: %s", err)
	}
}
