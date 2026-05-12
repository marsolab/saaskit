package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/heartwilltell/scotty"
	"github.com/marsolab/servekit/logkit"

	"github.com/marsolab/saaskit/back/internal/api"
	"github.com/marsolab/saaskit/back/internal/config"
)

func main() {
	cmd := scotty.Command{
		Name: "api",
		Run: func(cmd *scotty.Command, _ []string) error {
			ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer cancel()

			cfg, ok := scotty.GetConfig[config.Config](cmd)
			if !ok {
				return fmt.Errorf("no config in command %s", cmd.Name)
			}

			logger, err := initLogger(*cfg)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}

			logger.DebugContext(ctx, "Logger has been initialized",
				slog.String("level", cfg.LogLevel),
			)

			server, err := api.New(cfg, logger)
			if err != nil {
				return fmt.Errorf("build api server: %w", err)
			}

			return server.Serve(ctx)
		},
	}

	if err := config.Bind(&cmd); err != nil {
		fmt.Printf("command failed: %s", err.Error())
		os.Exit(1)
	}

	if err := cmd.Exec(); err != nil {
		fmt.Printf("command failed: %s", err.Error())
		os.Exit(1)
	}
}

// initLogger initializes the logger.
func initLogger(cfg config.Config) (*slog.Logger, error) {
	level, err := logkit.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}

	options := []logkit.Option{logkit.WithLevel(level)}

	if cfg.LogColor {
		options = append(options, logkit.WithColor())
	}

	if cfg.LogJSON {
		options = append(options, logkit.WithJSON())
	}

	return logkit.New(options...), nil
}
