// sanity-check is a CLI tool to run boot sanity checks for HelixAgent
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dev.helix.agent/internal/sanity"
)

func main() {
	var (
		host       string
		port       int
		pgHost     string
		pgPort     int
		redisHost  string
		redisPort  int
		cogneeHost string
		cogneePort int
		skipExt    bool
		jsonOutput bool
	)

	flag.StringVar(&host, "host", "localhost", "HelixAgent host")
	flag.IntVar(&port, "port", 7061, "HelixAgent port")
	flag.StringVar(&pgHost, "pg-host", "localhost", "PostgreSQL host")
	flag.IntVar(&pgPort, "pg-port", 5432, "PostgreSQL port")
	flag.StringVar(&redisHost, "redis-host", "localhost", "Redis host")
	flag.IntVar(&redisPort, "redis-port", 6379, "Redis port")
	flag.StringVar(&cogneeHost, "cognee-host", "localhost", "Cognee host")
	flag.IntVar(&cogneePort, "cognee-port", 8000, "Cognee port")
	flag.BoolVar(&skipExt, "skip-external", false, "Skip external provider checks")
	flag.BoolVar(&jsonOutput, "json", false, "Output as JSON")
	flag.Parse()

	config := &sanity.BootCheckConfig{
		HelixAgentHost:     host,
		HelixAgentPort:     port,
		PostgresHost:       pgHost,
		PostgresPort:       pgPort,
		RedisHost:          redisHost,
		RedisPort:          redisPort,
		CogneeHost:         cogneeHost,
		CogneePort:         cogneePort,
		SkipExternalChecks: skipExt,
	}

	report := sanity.RunSanityCheck(config)

	if jsonOutput {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
	}

	// Exit with appropriate code
	if !report.ReadyToStart {
		os.Exit(1)
	}
}
