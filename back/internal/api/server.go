package api

import (
	"fmt"
	"log/slog"

	"github.com/heartwilltell/hc"
	"github.com/marsolab/saaskit/back/internal/config"
	"github.com/marsolab/servekit"
	"github.com/marsolab/servekit/httpkit"
)

// New creates a new API server.
func New(cfg *config.Config, logger *slog.Logger) (servekit.Listener, error) {
	checker := hc.NewMultiServiceChecker(hc.NewServiceReport())

	listenerHTTP, listenerHTTPErr := httpkit.NewListenerHTTP(cfg.Addr,
		httpkit.WithLogger(logger),
		httpkit.WithMetrics(),
		httpkit.WithHealthCheck(
			httpkit.HealthChecker(checker),
			httpkit.HealthCheckReportHTML(),
		),
		httpkit.WithGlobalMiddlewares(
			httpkit.RecoveryMiddleware(),
			httpkit.MetricsMiddleware(),
			httpkit.LoggingMiddleware(logger),
		),
	)
	if listenerHTTPErr != nil {
		return nil, fmt.Errorf("create HTTP listener: %w", listenerHTTPErr)
	}

	// TODO: Add gRPC listener here if needed.

	server := servekit.NewServer(logger)
	server.RegisterListener("http", listenerHTTP)

	return server, nil
}
