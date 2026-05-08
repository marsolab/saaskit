package config

import (
	"fmt"

	"github.com/heartwilltell/scotty"
)

// Config represents the configuration for the application.
type Config struct {
	Addr string `flag:"addr" env:"ADDR" default:":8080" usage:"set HTTP listener address"`

	LogLevel string `flag:"log-level" env:"LOG_LEVEL" default:"info" usage:"set log level"`
	LogJSON  bool   `flag:"log-json" env:"LOG_JSON" default:"false" usage:"set log format to JSON"`
	LogColor bool   `flag:"log-color" env:"LOG_COLOR" default:"true" usage:"set colorful log output. Ignored if -log-json=true"`

	// TODO: add more config fields here.
}

// Bind binds config to a command.
func Bind(cmd *scotty.Command) error {
	var c Config

	if err := cmd.BindConfig(&c); err != nil {
		return fmt.Errorf("bind config to a command %s: %w", cmd.Name, err)
	}

	return nil
}
