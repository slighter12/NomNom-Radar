package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"radar/config"
	"radar/internal/delivery"
	apimiddleware "radar/internal/delivery/api/middleware"
	"radar/internal/delivery/api/router"
	"radar/internal/delivery/api/validator"
	"radar/internal/delivery/middleware"
	"radar/internal/domain/lifecycle"

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

var (
	// IEC units (KiB, Mi, ...) are powers of 1024.
	bodyLimitBinaryPattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s?([KMGTPE])iB?$`)
	// SI units (K, KB, M, MB, ...) are powers of 1000; a bare number or "B" is bytes.
	bodyLimitDecimalPattern = regexp.MustCompile(`(?i)^(\d+(?:\.\d+)?)\s?([KMGTPE]B?|B?)$`)
)

// parseBodyLimit converts a configured size such as "100KB" into bytes. Echo v5
// BodyLimit accepts only a byte count, so this reproduces the string grammar
// Echo v4 delegated to labstack/gommon/bytes: "1KB" and "1M" are decimal
// (1000-based) while "1KiB" and "1Mi" are binary (1024-based).
func parseBodyLimit(raw string) (int64, error) {
	base := 1000.0
	parts := bodyLimitDecimalPattern.FindStringSubmatch(raw)
	if binaryParts := bodyLimitBinaryPattern.FindStringSubmatch(raw); binaryParts != nil {
		base, parts = 1024.0, binaryParts
	}
	if parts == nil {
		return 0, fmt.Errorf("invalid http.maxRequestBodySize %q", raw)
	}

	amount, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid http.maxRequestBodySize %q: %w", raw, err)
	}

	multiplier := 1.0
	if unit := strings.ToUpper(parts[2]); unit != "" {
		// "B" is plain bytes and yields exponent 0, i.e. a multiplier of 1.
		multiplier = math.Pow(base, float64(strings.IndexByte("KMGTPE", unit[0])+1))
	}
	if amount*multiplier >= math.MaxInt64 {
		return 0, fmt.Errorf("invalid http.maxRequestBodySize %q: too large", raw)
	}

	return int64(amount * multiplier), nil
}
