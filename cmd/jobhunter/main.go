package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"jobhunter/internal/config"
	"jobhunter/internal/pipeline"
)

func main() {
	log.SetFlags(log.LstdFlags)
	log.SetPrefix("jobhunter ")

	configPath := flag.String("config", "config.yaml", "path to config.yaml")
	once := flag.Bool("once", false, "run one check and exit")
	dryRun := flag.Bool("dry-run", false, "write emails to data/outbox instead of sending them")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *dryRun {
		cfg.DryRun = true
	}

	lock, err := acquireLock(cfg.LockPath)
	if err != nil {
		log.Fatal(err)
	}
	defer lock.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Printf("watching for %s <%s>, every %s, lookback %s", cfg.User.Name, cfg.User.Email, cfg.Interval, cfg.Lookback)
	if cfg.DryRun {
		log.Printf("delivery is off (dry-run). Set SMTP_PASSWORD in .env to send real mail.")
	} else {
		log.Printf("delivery is on via %s:%d as %s", cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From)
	}

	for {
		started := time.Now()
		if err := pipeline.Run(ctx, cfg); err != nil {
			log.Printf("run failed: %v", err)
		} else {
			log.Printf("run finished in %s", time.Since(started).Round(time.Second))
		}
		if *once || ctx.Err() != nil {
			return
		}
		log.Printf("next check at %s", time.Now().Add(cfg.Interval).Format(time.RFC1123))
		timer := time.NewTimer(cfg.Interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			log.Printf("stopping")
			return
		case <-timer.C:
		}
	}
}

func acquireLock(path string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		file.Close()
		return nil, fmt.Errorf("another jobhunter process is already running")
	}
	return file, nil
}
