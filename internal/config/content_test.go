package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadRuntimeAndContentSettings(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "content"), 0700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(file, []byte("{}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(root, "content/site.yaml")
	data := "schema_version: 1\ntitle: 测试博客\nsubtitle: 你好\ncategories:\n  tech:\n    name: 技术\ntags:\n  go:\n    name: Go\n"
	if err := os.WriteFile(settings, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.ListenAddr != "127.0.0.1:25646" || c.Site.Title != "测试博客" || c.Build.PageSize != 10 || time.Duration(c.Build.ScanInterval) != time.Minute || time.Duration(c.Stats.ReportInterval) != 5*time.Minute || c.Stats.Details.Enabled || time.Duration(c.Stats.Details.Retention) != 168*time.Hour {
		t.Fatalf("defaults: %+v", c)
	}
	if c.Paths.Source != filepath.Join(root, "content") || c.Paths.Assets != filepath.Join(root, "content/assets") || c.Paths.Data != filepath.Join(root, "data") {
		t.Fatal("paths must resolve from config root")
	}
	for _, bad := range []string{data + "titel: bad\n", data + "title: duplicate\n", strings.Replace(data, "name: 技术", "name: 技术\n    parent: tech", 1)} {
		if err := os.WriteFile(settings, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadContentSettings(settings); err == nil {
			t.Fatal("invalid settings accepted")
		}
	}
	if err := os.WriteFile(settings, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("site:\n  title: old schema\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Fatal("legacy presentation in runtime config accepted")
	}
}
