package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"

	"radar/config"
	"radar/internal/delivery"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router"
	"radar/internal/delivery/api/validator"
	"radar/internal/delivery/middleware"
	"radar/internal/domain/lifecycle"

	"github.com/dustin/go-humanize"
	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"go.uber.org/fx"
)

type apiServer struct {
	cfg    *config.Config
	logger *slog.Logger
	server *echo.Echo
	// httpServer owns the listener lifecycle; Echo v5 no longer exposes one.
	httpServer *http.Server
}

// ServerParams holds dependencies for HTTP server, injected by Fx.
type ServerParams struct {
	fx.In

	Lc           fx.Lifecycle
	Cfg          *config.Config
	Logger       *slog.Logger
	RouterParams router.RouterParams
}

func NewServer(params ServerParams) (delivery.Delivery, error) {
	echoServer := echo.New()
	echoServer.IPExtractor = apimiddleware.NewClientIPExtractor(params.Cfg)

	// Set up middleware in correct order
	// 1. Recover middleware first (to catch panics early)
	echoServer.Use(echomiddleware.Recover())

	// 2. Request ID middleware (must be before logger to include in logs)
	requestIDMiddleware := middleware.NewRequestIDMiddleware(params.Logger)
	echoServer.Use(requestIDMiddleware.Process)

	// Set up centralized error handler
	errorMiddleware := apimiddleware.NewErrorMiddleware(params.Logger)
	echoServer.HTTPErrorHandler = errorMiddleware.HandleHTTPError

	// 3. Request lifecycle logger
	requestLogger := apimiddleware.NewRequestLoggerMiddleware(params.Logger, params.Cfg)
	echoServer.Use(requestLogger.Log)

	// 4. Convert returned errors and inner panics into HTTP responses inside the middleware chain.
	echoServer.Use(errorMiddleware.HandleErrors)
	echoServer.Use(errorMiddleware.RecoverPanics)

	// 5. Domain guard middleware
	domainGuard := apimiddleware.NewDomainGuardMiddleware(params.Cfg)
	echoServer.Use(domainGuard.ValidateHost)

	// 6. CORS middleware
	corsMiddleware, err := apimiddleware.NewCORSMiddleware(params.Cfg)
	if err != nil {
		return nil, fmt.Errorf("configure CORS middleware: %w", err)
	}
	if corsMiddleware != nil {
		echoServer.Use(corsMiddleware)
	}

	// 7. Request body size limit
	bodyLimit, err := parseBodyLimit(params.Cfg.HTTP.MaxRequestBodySize)
	if err != nil {
		return nil, fmt.Errorf("configure request body limit: %w", err)
	}
	echoServer.Use(echomiddleware.BodyLimit(bodyLimit))

	// 8. Keep a bounded JSON body copy for sanitized error-only request logging.
	echoServer.Use(apimiddleware.CaptureRequestBodyForErrorLog)

	// Set up validator
	echoServer.Validator = validator.New()

	r := router.NewRouter(params.RouterParams)
	if err := r.RegisterRoutes(echoServer); err != nil {
		return nil, fmt.Errorf("register API routes: %w", err)
	}
	r.RegisterTestRoutes(echoServer)

	srv := &apiServer{
		cfg:        params.Cfg,
		logger:     params.Logger,
		server:     echoServer,
		httpServer: newHTTPServer(echoServer, params.Cfg, params.Logger),
	}

	params.Lc.Append(fx.Hook{
		OnStop: srv.stop,
	})

	return srv, nil
}

// newHTTPServer builds the standard library server that fronts Echo. It serves
// HTTP/1 and unencrypted HTTP/2 (h2c) on the same port via the standard
// library, avoiding the deprecated golang.org/x/net/http2 API.
func newHTTPServer(handler http.Handler, cfg *config.Config, logger *slog.Logger) *http.Server {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	return &http.Server{
		Handler:           handler,
		ReadTimeout:       cfg.HTTP.Timeouts.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.Timeouts.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.Timeouts.WriteTimeout,
		IdleTimeout:       cfg.HTTP.Timeouts.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		Protocols:         &protocols,
	}
}

func (s *apiServer) Serve(ctx context.Context) error {
	hostPort := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.cfg.HTTP.Port))
	s.logger.Info("Starting API HTTP server", slog.String("host_port", hostPort))

	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", hostPort)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", hostPort, err)
	}

	if err := s.httpServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve %s HTTP: %w", "'+name+'", err)
	}

	return nil
}

func (s *apiServer) stop(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, lifecycle.DefaultTimeout)
	defer cancel()

	s.logger.Info("Shutting down API HTTP server")

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown %s HTTP server: %w", "'+name+'", err)
	}

	return nil
}

// parseBodyLimit converts a configured size such as "100KB" into bytes. Echo v5
// BodyLimit takes a byte count, while Echo v4 parsed the string itself: "1KB"
// and "1M" are decimal (1000-based), "1KiB" and "1Mi" binary (1024-based), and
// humanize.ParseBytes keeps those semantics. Zero is rejected because v4 meant
// "unlimited chunked body" and v5 means "reject every body"; neither is useful.
func parseBodyLimit(raw string) (int64, error) {
	size, err := humanize.ParseBytes(raw)
	if err != nil || size == 0 || size > math.MaxInt64 {
		return 0, fmt.Errorf("invalid http.maxRequestBodySize %q: want 1 byte to %d bytes", raw, int64(math.MaxInt64))
	}

	return int64(size), nil
}
