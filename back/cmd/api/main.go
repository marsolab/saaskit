package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"

	"github.com/heartwilltell/scotty"
	"github.com/marsolab/saaskit/back/internal/config"
	"github.com/marsolab/servekit/logkit"
)

func main() {
	cmd := scotty.Command{
		Name: "api",
		Run: func(cmd *scotty.Command, args []string) error {
			ctx, cancel := signal.NotifyContext(context.Background())
			defer cancel()

			cfg, ok := scotty.GetConfig[config.Config](cmd)
			if !ok {
				return fmt.Errorf("no config in command %s", cmd.Name)
			}

			logger, loggerErr := initLogger(*cfg)
			if loggerErr != nil {
				return fmt.Errorf("init logger: %w", loggerErr)
			}

			logger.DebugContext(ctx, "Logger has been initialized",
				slog.String("level", cfg.LogLevel),
			)

			return nil
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
	level, levelErr := logkit.ParseLevel(cfg.LogLevel)
	if levelErr != nil {
		return nil, fmt.Errorf("parse log level: %w", levelErr)
	}

	loggerOptions := []logkit.Option{
		logkit.WithLevel(level),
		logkit.WithColor(),
	}

	if cfg.LogJSON {
		loggerOptions = append(loggerOptions, logkit.WithJSON())
	}

	logger := logkit.New(loggerOptions...)

	return logger, nil
}
