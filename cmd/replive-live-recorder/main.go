package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"replive/config"
	"replive/rep_api"
	"replive/service"
	"replive/utils"
	"syscall"
	"time"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	interval := flag.Duration("interval", 2*time.Second, "live status polling interval")
	flag.Parse()

	utils.UseJapanLocalTime()
	if err := initRecorder(*configPath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := service.RunLiveRecorder(ctx, *interval); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initRecorder(configPath string) error {
	if err := config.EnsureConfig(configPath); err != nil {
		return fmt.Errorf("ensure config failed: %w", err)
	}
	if err := config.LoadConfig(configPath); err != nil {
		return fmt.Errorf("load config failed: %w", err)
	}
	if !config.HasRefreshToken() {
		return fmt.Errorf("refresh_token is missing; run replive-plus once to complete login before starting the live recorder")
	}
	if err := rep_api.InitHttp(); err != nil {
		return fmt.Errorf("initialize Replive API failed: %w", err)
	}
	return nil
}
