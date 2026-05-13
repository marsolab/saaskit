package api

import (
	"fmt"
	"log/slog"

	"github.com/heartwilltell/hc"
	"github.com/marsolab/servekit"
	"github.com/marsolab/servekit/grpckit"
	"github.com/marsolab/servekit/httpkit"

	"github.com/marsolab/saaskit/back/internal/api/service/authkinde"
	"github.com/marsolab/saaskit/back/internal/config"
)

// New creates a new API server. The authkinde transport is only mounted when
// Kinde credentials are configured; otherwise the server still boots so local
// development can run against a Kinde-less stack.
func New(cfg *config.Config, logger *slog.Logger) (servekit.Listener, error) {
	checker := hc.NewMultiServiceChecker(hc.NewServiceReport())
	server := servekit.NewServer(logger)

	listenerHTTP, err := httpkit.NewListenerHTTP(cfg.Addr,
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
	if err != nil {
		return nil, fmt.Errorf("create HTTP listener: %w", err)
	}

	if kindeCfg := cfg.Kinde(); kindeCfg.Domain != "" {
		authSvc, authErr := authkinde.NewService(kindeCfg)
		if authErr != nil {
			return nil, fmt.Errorf("build authkinde service: %w", authErr)
		}

		authTransport := authkinde.NewTransportHTTP(authSvc, logger, authkinde.TransportOptions{
			CookieDomain: kindeCfg.CookieDomain,
		})

		listenerHTTP.Mount("/v1/auth", authTransport)
	} else {
		logger.Warn("authkinde: Kinde domain not configured, auth routes are not mounted")
	}

	server.RegisterListener("http", listenerHTTP)

	listenerGRPC, err := grpckit.NewListenerGRPC(cfg.AddrGRPC,
		grpckit.WithLogger(logger),
		grpckit.WithUnaryInterceptors(
			grpckit.LoggingInterceptor(logger),
			grpckit.MetricsInterceptor(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("create gRPC listener: %w", err)
	}

	server.RegisterListener("grpc", listenerGRPC)

	return server, nil
}
