package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/filelock"
	"github.com/lsy223622/BloGo/internal/site"
)

type watcher struct {
	configFile, target, lastSuccessful, failedFingerprint, lastLogKey string
	lastLogAt                                                         time.Time
	interval                                                          time.Duration
	log                                                               io.Writer
}

func newWatcher(c *config.Config, log io.Writer) *watcher {
	w := &watcher{configFile: c.FilePath(), target: c.Paths.Current, interval: time.Duration(c.Build.ScanInterval), log: log}
	if manifest, err := site.ReadManifest(filepath.Join(c.Paths.Current, "meta/manifest.json")); err == nil {
		w.lastSuccessful = manifest.Fingerprint
	}
	return w
}

func (w *watcher) report(err error, key string) {
	if key != w.lastLogKey || time.Since(w.lastLogAt) >= time.Minute {
		fmt.Fprintf(w.log, "watch: %v\n", err)
		w.lastLogKey = key
		w.lastLogAt = time.Now()
	}
}

func (w *watcher) step() (*Result, error) {
	c, err := config.Load(w.configFile)
	if err != nil {
		w.report(err, "config:"+err.Error())
		return nil, err
	}
	w.interval = time.Duration(c.Build.ScanInterval)
	if c.Paths.Current != w.target {
		err = fmt.Errorf("current target changed; restart watcher")
		w.report(err, err.Error())
		return nil, err
	}
	if err = c.Validate("watch", ""); err != nil {
		w.report(err, err.Error())
		return nil, err
	}
	fingerprint, err := Scan(c, false)
	if err != nil {
		w.report(err, err.Error())
		return nil, err
	}
	if fingerprint == w.lastSuccessful || fingerprint == w.failedFingerprint {
		return nil, nil
	}
	result, err := publishLocked(c, fingerprint)
	if err != nil {
		var contentError *ValidationError
		if errors.As(err, &contentError) {
			w.failedFingerprint = fingerprint
		}
		w.report(err, fingerprint+":"+err.Error())
		return nil, err
	}
	w.lastSuccessful = result.Manifest.Fingerprint
	w.failedFingerprint = ""
	w.lastLogKey = ""
	for _, warning := range result.Warnings {
		fmt.Fprintf(w.log, "warning: %s\n", warning)
	}
	fmt.Fprintf(w.log, "Published %s\n", result.Manifest.BuildID)
	return result, nil
}

func Watch(ctx context.Context, c *config.Config, log io.Writer) error {
	if err := publicationSupported(); err != nil {
		return err
	}
	if err := c.Validate("watch", ""); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.Paths.Current), 0755); err != nil {
		return err
	}
	lock, err := filelock.Acquire(c.Paths.Current + ".lock")
	if err != nil {
		return err
	}
	defer lock.Close()
	w := newWatcher(c, log)
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			_, _ = w.step()
			timer.Reset(w.interval)
		}
	}
}
