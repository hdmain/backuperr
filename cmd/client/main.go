package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"backuperr/internal/client"
)

func init() {
	// Use every CPU the OS reports (respects cgroup/cpuset on Linux).
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func main() {
	cfgPath := flag.String("config", "client.yaml", "path to YAML configuration")
	flag.Parse()
	args := flag.Args()

	cfg, err := client.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	base, err := cfg.BaseURL()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.APIKey == "" {
		log.Fatal("config: api_key is required")
	}

	api := &client.API{BaseURL: base, APIKey: cfg.APIKey}

	cfgAbs, err := filepath.Abs(*cfgPath)
	if err != nil {
		log.Fatalf("config path: %v", err)
	}

	if len(args) == 0 {
		for {
			act, err := client.RunRootMenu(api)
			if err != nil {
				log.Fatalf("menu: %v", err)
			}
			switch act {
			case client.ActionQuit:
				return
			case client.ActionBackupIncremental:
				runBackup(cfg, api, false)
			case client.ActionBackupFull:
				runBackup(cfg, api, true)
			case client.ActionRestore:
				runRestore(cfg, api)
			case client.ActionList:
				runList(api)
			case client.ActionSchedule:
				if err := client.RunScheduleWizard(cfgAbs); err != nil {
					log.Printf("schedule: %v", err)
				}
			}
		}
	}

	switch args[0] {
	case "backup":
		full := flag.NewFlagSet("backup", flag.ExitOnError)
		forceFull := full.Bool("full", false, "force a full backup")
		_ = full.Parse(args[1:])
		runBackup(cfg, api, *forceFull)
	case "restore":
		runRestore(cfg, api)
	case "list":
		runList(api)
	default:
		printUsage()
		os.Exit(2)
	}
}

func runBackup(cfg *client.Config, api *client.API, forceFull bool) {
	meta, err := client.RunBackup(cfg, api, forceFull)
	if err != nil {
		log.Printf("backup: %v", err)
		client.SendClientWebhook(cfg, api, "error", err.Error(), "backup_failed")
		return
	}
	client.SendClientWebhook(cfg, api, "ok",
		fmt.Sprintf("type=%s id=%s files=%d bytes=%d", meta.Type, meta.ID, meta.FileCount, meta.Bytes),
		"backup_complete")
	fmt.Printf("backup %s (%s) id=%s files=%d bytes=%d\n",
		meta.Type, meta.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"), meta.ID, meta.FileCount, meta.Bytes)
}

func runRestore(cfg *client.Config, api *client.API) {
	list, err := api.ListBackups()
	if err != nil {
		log.Printf("list: %v", err)
		return
	}
	id, err := client.RunPickBackup(list)
	if err != nil {
		log.Printf("tui: %v", err)
		return
	}
	if id == "" {
		fmt.Println("cancelled")
		return
	}
	if err := client.RunRestore(cfg, api, id); err != nil {
		log.Printf("restore: %v", err)
		client.SendClientWebhook(cfg, api, "error", err.Error(), "restore_failed")
		return
	}
	client.SendClientWebhook(cfg, api, "ok", fmt.Sprintf("restored id=%s", id), "restore_complete")
}

func runList(api *client.API) {
	list, err := api.ListBackups()
	if err != nil {
		log.Printf("list: %v", err)
		return
	}
	for _, b := range list {
		at := b.CreatedAt.UTC()
		when := fmt.Sprintf("%s (%s)", at.Format("2006-01-02 15:04 UTC"), client.HumanTimeRel(at))
		fmt.Printf("%s\t%s\t%s\t%d\t%d\n", b.ID, b.Type, when, b.FileCount, b.Bytes)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  client [-config path.yaml]                 interactive menu (default; includes cron schedule on Linux)
  client [-config path.yaml] backup [--full]
  client [-config path.yaml] restore
  client [-config path.yaml] list
`)
}
