package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"time"

	"github.com/lsy223622/BloGo/internal/build"
	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/filelock"
	"github.com/lsy223622/BloGo/internal/stats"
)

type service struct {
	config, original     *config.Config
	result               *build.Result
	handler              *Handler
	store                *stats.Store
	key                  []byte
	collector            http.Handler
	lock                 *filelock.Lock
	preview              bool
	output, listen       string
	log                  io.Writer
	lastError            string
	nextScan, nextReport time.Time
}

func Serve(ctx context.Context, c *config.Config, log io.Writer) error {
	if runtime.GOOS != "linux" {
		return fmt.Errorf("serve requires Linux; use dev or build on this platform")
	}
	return runService(ctx, c, false, "", c.Server.ListenAddr, log)
}

func Dev(ctx context.Context, c *config.Config, output, listen string, log io.Writer) error {
	if err := config.Listener(listen); err != nil {
		return err
	}
	return runService(ctx, c, true, output, listen, log)
}

func runService(ctx context.Context, c *config.Config, preview bool, output, listen string, log io.Writer) error {
	if log == nil {
		log = io.Discard
	}
	r, err := startRuntime(ctx, c, preview, output, listen, log)
	if err != nil {
		return err
	}
	defer r.close()
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	server := &http.Server{Handler: r.handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	fmt.Fprintf(log, "BloGo listening on http://%s (%s)\n", listener.Addr(), r.result.Output)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	defer func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			server.Close()
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-done:
			if err == http.ErrServerClosed {
				return nil
			}
			return err
		case now := <-ticker.C:
			r.step(ctx, now)
		}
	}
}

func runtimeConfig(c *config.Config, preview bool, listen string) *config.Config {
	result := *c
	if preview {
		result.Site.BaseURL = "http://" + listen
		root := filepath.Join(c.Paths.Data, "dev")
		result.Paths.Data = root
		result.Paths.Releases = filepath.Join(root, "releases")
		result.Paths.Staging = filepath.Join(root, "staging")
		result.Paths.Current = filepath.Join(root, "current")
		result.Stats.DatabasePath = filepath.Join(root, "statistics/stats.db")
		result.Stats.VisitorKeyPath = filepath.Join(root, "statistics/visitor.key")
		result.Stats.PublicOutput = filepath.Join(root, "public/stats.json")
		result.Stats.PrivateOutput = filepath.Join(root, "private/report.html")
		result.Stats.ManifestPath = filepath.Join(root, "collector/manifest.json")
	}
	return &result
}

func startRuntime(ctx context.Context, c *config.Config, preview bool, output, listen string, log io.Writer) (_ *service, err error) {
	model := runtimeConfig(c, preview, listen)
	validate, validationOutput := "publish", ""
	if preview {
		validate = "dev"
		validationOutput = filepath.Join(model.Paths.Data, "previews")
		if output != "" {
			validationOutput = output
		}
	}
	if err = model.Validate(validate, validationOutput); err != nil {
		return nil, err
	}
	if err = os.MkdirAll(model.Paths.Data, 0700); err != nil {
		return nil, err
	}
	lock, err := filelock.Acquire(filepath.Join(model.Paths.Data, "run.lock"))
	if err != nil {
		return nil, err
	}
	r := &service{config: model, original: c, lock: lock, preview: preview, output: output, listen: listen, log: log}
	defer func() {
		if err != nil {
			r.close()
		}
	}()
	// Verify existing identity before any operation can create a new key.
	if info, e := os.Stat(model.Stats.DatabasePath); e == nil && info.Size() > 0 {
		if _, e = os.Stat(model.Stats.VisitorKeyPath); e != nil {
			return nil, fmt.Errorf("existing database requires its original visitor key: %w", e)
		}
	}
	if !preview {
		r.result, _ = build.ReadPublished(model.Paths.Current)
	}
	if r.result == nil {
		r.result, err = r.build(model)
		if err != nil {
			return nil, err
		}
	}
	if preview {
		if err = r.writeCollectorManifest(r.result); err != nil {
			return nil, err
		}
	} else {
		r.config.Stats.ManifestPath = filepath.Join(r.result.Output, "meta/manifest.json")
	}
	if err = r.configureCollector(ctx); err != nil {
		return nil, err
	}
	r.handler, err = newHandler(r.config, r.result, r.collector, preview)
	if err != nil {
		return nil, err
	}
	if r.config.Stats.Enabled {
		if err = r.report(ctx); err != nil {
			return nil, err
		}
	}
	now := time.Now()
	r.nextScan = now
	r.nextReport = now.Add(time.Duration(model.Stats.ReportInterval))
	return r, nil
}

func (r *service) close() {
	if r.store != nil {
		r.store.Close()
	}
	if r.lock != nil {
		r.lock.Close()
	}
}

func (r *service) build(c *config.Config) (*build.Result, error) {
	if !r.preview {
		safe := *c
		if safe.Build.KeepReleases < 2 {
			safe.Build.KeepReleases = 2
		}
		return build.Publish(&safe)
	}
	root := r.output
	if root == "" {
		root = filepath.Join(c.Paths.Data, "previews")
	} else if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(c.FilePath()), root)
	}
	// Each snapshot has a new path: Windows never has to rename a served directory.
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	candidate, err := os.MkdirTemp(root, "preview-")
	if err != nil {
		return nil, err
	}
	result, err := build.Build(c, candidate, true)
	if err != nil {
		os.RemoveAll(candidate)
	}
	return result, err
}

func (r *service) writeCollectorManifest(result *build.Result) error {
	manifest := *result.Manifest
	manifest.Pages = nil
	for _, p := range result.Manifest.Pages {
		if p.Visibility != content.Draft {
			manifest.Pages = append(manifest.Pages, p)
		}
	}
	if err := manifest.Validate(false); err != nil {
		return err
	}
	data, err := json.Marshal(&manifest)
	if err != nil {
		return err
	}
	target := r.config.Stats.ManifestPath
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(target), ".manifest-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err = temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), target)
}

func (r *service) configureCollector(ctx context.Context) error {
	if !r.config.Stats.Enabled {
		r.collector = nil
		return nil
	}
	if r.store == nil {
		if info, e := os.Stat(r.config.Stats.DatabasePath); e == nil && info.Size() > 0 {
			if _, e = os.Stat(r.config.Stats.VisitorKeyPath); e != nil {
				return fmt.Errorf("existing database requires original visitor key: %w", e)
			}
		}
		var err error
		r.key, err = stats.LoadOrCreateKey(r.config.Stats.VisitorKeyPath)
		if err != nil {
			return err
		}
		initialize, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		r.store, err = stats.OpenWriter(initialize, r.config.Stats.DatabasePath)
		if err != nil {
			return err
		}
	}
	collector, err := stats.NewHandler(r.config, r.store, r.key)
	if err != nil {
		return err
	}
	r.collector = collector
	return nil
}

func (r *service) report(ctx context.Context) error {
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	zone, _ := time.LoadLocation(r.config.Site.Timezone)
	if err := r.store.Cleanup(bounded, time.Now(), time.Duration(r.config.Stats.VisitorRetention), zone, time.Duration(r.config.Stats.Details.Retention)); err != nil {
		return err
	}
	return stats.GenerateReport(bounded, r.config)
}

func restartRequired(a, b *config.Config) bool {
	return a.Server.ListenAddr != b.Server.ListenAddr || a.Server.BaseURL != b.Server.BaseURL || a.Timezone != b.Timezone || a.Paths != b.Paths
}

func (r *service) diagnostic(err error) {
	if err != nil && err.Error() != r.lastError {
		fmt.Fprintf(r.log, "blogo: %v\n", err)
		r.lastError = err.Error()
	}
}

func (r *service) step(ctx context.Context, now time.Time) {
	// Configuration is checked independently so shortening a long scan interval takes effect promptly.
	loaded, err := config.Load(r.original.FilePath())
	if err != nil {
		r.diagnostic(err)
	} else if restartRequired(r.original, loaded) {
		r.diagnostic(fmt.Errorf("runtime paths, listener, base_url or timezone changed; restart required"))
	} else {
		next := runtimeConfig(loaded, r.preview, r.listen)
		next.Stats.ManifestPath = r.config.Stats.ManifestPath
		if !reflect.DeepEqual(r.config.Stats, next.Stats) || !reflect.DeepEqual(r.config.Server.TrustedProxies, next.Server.TrustedProxies) {
			previous, collector := r.config, r.collector
			r.config = next
			if err = r.configureCollector(ctx); err != nil {
				r.config, r.collector = previous, collector
				r.diagnostic(err)
			} else {
				r.handler.Reconfigure(r.config, r.result, r.collector)
				r.nextReport = now
			}
		} else {
			r.config = next
		}
		if loaded.Build.ScanInterval != r.original.Build.ScanInterval {
			r.nextScan = now
		}
		if loaded.Stats.ReportInterval != r.original.Stats.ReportInterval {
			r.nextReport = now
		}
		r.original = loaded
	}
	if !now.Before(r.nextScan) {
		r.nextScan = now.Add(time.Duration(r.config.Build.ScanInterval))
		fingerprint, e := build.Scan(r.config, r.preview)
		if e != nil {
			r.diagnostic(e)
		} else if fingerprint != r.result.Manifest.Fingerprint {
			updated, e := r.build(r.config)
			if e != nil {
				r.diagnostic(e)
			} else {
				old := r.result
				if r.preview {
					e = r.writeCollectorManifest(updated)
				} else {
					clone := *r.config
					clone.Stats.ManifestPath = filepath.Join(updated.Output, "meta/manifest.json")
					r.config = &clone
				}
				if e == nil {
					e = r.configureCollector(ctx)
				}
				if e == nil {
					e = r.handler.Reconfigure(r.config, updated, r.collector)
				}
				if e != nil {
					r.diagnostic(e)
				} else {
					r.result = updated
					r.lastError = ""
					r.nextReport = now
					fmt.Fprintf(r.log, "Published %s\n", updated.Manifest.BuildID)
					if r.preview {
						if old.Output != updated.Output {
							if e = os.RemoveAll(old.Output); e != nil {
								r.diagnostic(e)
							}
						}
					} else {
						r.diagnostic(build.PruneReleases(r.config, updated.Output))
					}
				}
			}
		}
	}
	if r.config.Stats.Enabled && !now.Before(r.nextReport) {
		r.diagnostic(r.report(ctx))
		r.nextReport = now.Add(time.Duration(r.config.Stats.ReportInterval))
	}
}
