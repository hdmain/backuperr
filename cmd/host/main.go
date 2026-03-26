package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"runtime"

	"backuperr/internal/host"
)

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	cfgPath := flag.String("config", "config.yml", "path to host YAML configuration")
	addr := flag.String("listen", ":8443", "HTTP listen address")
	dataDir := flag.String("data", "./data", "directory for backup storage")
	mainKey := flag.String("key", "", "main API key for client authorization (required)")
	flag.Parse()

	var webhookURL string
	// Load defaults from config.yml if present.
	if _, err := os.Stat(*cfgPath); err == nil {
		cfg, err := host.LoadConfig(*cfgPath)
		if err != nil {
			log.Fatalf("read host config: %v", err)
		}
		if cfg.Listen != "" {
			*addr = cfg.Listen
		}
		if cfg.DataDir != "" {
			*dataDir = cfg.DataDir
		}
		if cfg.MainKey != "" {
			*mainKey = cfg.MainKey
		}
		webhookURL = cfg.WebhookURL
	}

	if *mainKey == "" {
		log.Fatal("main key is required (set in config.yml or pass -key)")
	}
	if err := os.MkdirAll(*dataDir, 0o750); err != nil {
		log.Fatalf("data dir: %v", err)
	}

	srv := &host.Server{
		DataDir:    *dataDir,
		MainKey:    *mainKey,
		Log:        log.Default(),
		WebhookURL: webhookURL,
	}
	srv.NotifyStartupWebhook()
	log.Printf("backuperr host listening on %s, data=%s", *addr, *dataDir)
	if err := http.ListenAndServe(*addr, srv.Handler()); err != nil {
		log.Fatal(err)
	}
}
