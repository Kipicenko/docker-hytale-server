package main

import (
	"context"
	"docker-hytale-server/internal/config"
	"docker-hytale-server/internal/oauth"
	"docker-hytale-server/internal/runner"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/charmbracelet/log"
)

func main() {
	runner.LogIntro()
	config.Load()

	cfg := config.Get()

	ctxSignal, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var wg sync.WaitGroup

	runner.SettingsConfigJson()

	runner.DownloadServerFiles(ctxSignal)

	gameSession := oauth.GetGameSession(ctxSignal)

	if cfg.AutoRefreshTokens {
		log.Info("Automatic refresh tokens is enabled.")
		wg.Add(1)
		go oauth.AutoRefreshTokens(ctxSignal, &wg)
	}

	runner.StartHytaleServer(ctxSignal, gameSession)

	wg.Wait()

	log.Info(" ✅ The hytale server was successfully stopped.")
}
