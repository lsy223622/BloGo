package preview

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/server"
)

type Handler = server.Handler

func NewHandler(result *build.Result) (*Handler, error) { return server.NewPreviewHandler(result) }

func Dev(ctx context.Context, c *config.Config, output, listen string, log io.Writer) error {
	if err := config.Listener(listen); err != nil {
		return err
	}
	result, err := build.Build(c, output, true)
	if err != nil {
		return err
	}
	handler, err := NewHandler(result)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintf(log, "Preview http://%s (%s)\n", listener.Addr(), result.Output)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastSuccessful := result.Manifest.Fingerprint
	failed, lastError := "", ""
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			return server.Shutdown(shutdown)
		case err := <-done:
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		case <-ticker.C:
			model, err := config.Load(c.FilePath())
			if err != nil {
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			fingerprint, err := build.Scan(model, true)
			if err != nil {
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			if fingerprint == lastSuccessful || fingerprint == failed {
				continue
			}
			updated, err := build.Build(model, output, true)
			if err != nil {
				if _, ok := err.(*build.ValidationError); ok {
					failed = fingerprint
				}
				if err.Error() != lastError {
					fmt.Fprintf(log, "dev: %v\n", err)
					lastError = err.Error()
				}
				continue
			}
			if err = handler.Update(updated); err != nil {
				fmt.Fprintf(log, "dev: %v\n", err)
				continue
			}
			lastSuccessful = updated.Manifest.Fingerprint
			failed = ""
			lastError = ""
			fmt.Fprintf(log, "Rebuilt %s\n", updated.Manifest.BuildID)
		}
	}
}
