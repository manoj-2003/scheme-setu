// Command server runs the Scheme Setu API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/manoj-2003/scheme-setu/internal/catalog"
	"github.com/manoj-2003/scheme-setu/internal/config"
	"github.com/manoj-2003/scheme-setu/internal/fraud"
	"github.com/manoj-2003/scheme-setu/internal/freshness"
	"github.com/manoj-2003/scheme-setu/internal/handlers"
	"github.com/manoj-2003/scheme-setu/internal/serp"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("scheme-setu: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	cat, err := loadCatalog(cfg)
	if err != nil {
		return err
	}

	client, err := serp.New(cfg.SerpOptions())
	if err != nil {
		return fmt.Errorf("serp client: %w", err)
	}
	defer client.Close()

	h := &handlers.Handler{
		Catalog:             cat,
		Client:              client,
		Fresh:               freshness.New(client),
		Fraud:               fraud.New(client, cat.OfficialDomains()),
		DiscoveryLangs:      cfg.DiscoveryLangs,
		MaxDiscoveryQueries: cfg.MaxDiscoveryQueries,
		VerifyTopN:          cfg.VerifyTopN,
		RequestTimeout:      45 * time.Second,
	}

	e := echo.New()
	e.HideBanner = true
	e.Validator = &requestValidator{v: validator.New()}

	e.Use(middleware.RequestID())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogLatency:   true,
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogRequestID: true,
		LogError:     true,
		HandleError:  true,
		LogValuesFunc: func(_ echo.Context, v middleware.RequestLoggerValues) error {
			if v.Error != nil {
				log.Printf("%s %s -> %d (%s) id=%s error=%v",
					v.Method, v.URI, v.Status, v.Latency, v.RequestID, v.Error)
				return nil
			}
			log.Printf("%s %s -> %d (%s) id=%s", v.Method, v.URI, v.Status, v.Latency, v.RequestID)
			return nil
		},
	}))
	e.Use(middleware.BodyLimit("64K"))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORSOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
	}))

	h.Register(e)

	// Optionally serve the built frontend, so the whole app can run from a
	// single binary with no Node process.
	if cfg.WebDir != "" {
		if _, statErr := os.Stat(cfg.WebDir); statErr == nil {
			e.Static("/", cfg.WebDir)
			log.Printf("serving frontend from %s", cfg.WebDir)
		} else {
			log.Printf("WEB_DIR %q not found, skipping static frontend", cfg.WebDir)
		}
	}

	logStartup(cfg, cat, client)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := e.StartServer(srv); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case <-stop:
		log.Println("shutting down")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return e.Shutdown(ctx)
	}
}

func loadCatalog(cfg config.Config) (*catalog.Catalog, error) {
	if cfg.CatalogPath != "" {
		cat, err := catalog.LoadFile(cfg.CatalogPath)
		if err != nil {
			return nil, err
		}
		log.Printf("catalog loaded from %s", filepath.Clean(cfg.CatalogPath))
		return cat, nil
	}
	return catalog.Load()
}

func logStartup(cfg config.Config, cat *catalog.Catalog, client *serp.Client) {
	log.Printf("schemes=%d mode=%s cache=%s langs=%v",
		cat.Len(), client.Mode(), cfg.SerpCachePath, cfg.DiscoveryLangs)

	if client.Mode() == serp.ModeCacheOnly {
		if cfg.SerpAPIKey == "" {
			log.Println("SERPAPI_KEY is not set: running cache-only. Live discovery, " +
				"freshness and fraud checks are disabled and will be reported as unverified.")
		} else {
			log.Println("SERP_MODE=cache: serving only cached responses, no credits will be spent")
		}
	} else {
		log.Printf("live mode: at most %d SerpApi calls this process", cfg.SerpMaxCredits)
	}

	if n := len(cat.Unverified()); n > 0 {
		log.Printf("WARNING: %d/%d catalog entries still need human verification "+
			"against official sources", n, cat.Len())
	}
	log.Printf("listening on http://localhost:%s", cfg.Port)
}

// requestValidator adapts go-playground/validator to Echo's interface.
type requestValidator struct {
	v *validator.Validate
}

func (r *requestValidator) Validate(i any) error {
	if err := r.v.Struct(i); err != nil {
		return echo.NewHTTPError(http.StatusUnprocessableEntity, humanize(err))
	}
	return nil
}

// humanize turns validator's struct-tag errors into messages a frontend can
// show directly.
func humanize(err error) string {
	var verrs validator.ValidationErrors
	if !errors.As(err, &verrs) {
		return err.Error()
	}

	var msg strings.Builder
	for i, fe := range verrs {
		if i > 0 {
			msg.WriteString("; ")
		}
		switch fe.Tag() {
		case "required":
			fmt.Fprintf(&msg, "%s is required", fe.Field())
		case "oneof":
			fmt.Fprintf(&msg, "%s must be one of: %s", fe.Field(), fe.Param())
		case "gte":
			fmt.Fprintf(&msg, "%s must be at least %s", fe.Field(), fe.Param())
		case "lte":
			fmt.Fprintf(&msg, "%s must be at most %s", fe.Field(), fe.Param())
		default:
			fmt.Fprintf(&msg, "%s failed %s", fe.Field(), fe.Tag())
		}
	}
	return msg.String()
}
