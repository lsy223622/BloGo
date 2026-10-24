package stats

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/site"
)

func Serve(ctx context.Context, c *config.Config, log io.Writer) error {
	if !c.Stats.Enabled {
		return fmt.Errorf("statistics disabled")
	}
	if err := c.Validate("stats", ""); err != nil {
		return err
	}
	if _, err := site.ReadManifest(c.Stats.ManifestPath); err != nil {
		return err
	}
	listener, err := net.Listen("tcp", c.Stats.ListenAddr)
	if err != nil {
		return err
	}
	defer listener.Close()
	key, err := LoadOrCreateKey(c.Stats.VisitorKeyPath)
	if err != nil {
		return err
	}
	initialize, cancel := context.WithTimeout(ctx, 10*time.Second)
	store, err := OpenWriter(initialize, c.Stats.DatabasePath)
	cancel()
	if err != nil {
		return err
	}
	defer store.Close()
	handler, err := NewHandler(c, store, key)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintf(log, "Statistics listening on %s\n", listener.Addr())
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			return server.Shutdown(shutdown)
		case err := <-done:
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		case <-ticker.C:
			cleanup, stop := context.WithTimeout(ctx, 10*time.Second)
			err := store.Cleanup(cleanup, time.Now(), time.Duration(c.Stats.VisitorRetention), handler.zone, time.Duration(c.Stats.Details.Retention))
			stop()
			if err != nil {
				fmt.Fprintf(log, "statistics cleanup: %v\n", err)
			}
		}
	}
}
