package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, extra string) string {
	t.Helper()
	dir := t.TempDir()
	for _, sub := range []string{"content", "templates", "static"} {
		if err := os.Mkdir(filepath.Join(dir, sub), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "taxonomies.yaml"), []byte("schema_version: 1\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	data := "site:\n  base_url: https://example.com\n  title: 测试\npaths:\n  source: content\n  templates: templates\n  static: static\n" + extra
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoadDefaultsAndCommandBoundaries(t *testing.T) {
	file := fixture(t, "extra:\n  menu: [关于, 标签]\n")
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Site.Timezone != "Asia/Shanghai" || c.Site.Language != "zh-CN" || c.Build.PageSize != 10 || c.Build.SummaryMaxChars != 160 || c.Build.KeepReleases != 3 || time.Duration(c.Build.ScanInterval) != time.Minute {
		t.Fatalf("defaults: %+v", c)
	}
	if !filepath.IsAbs(c.Paths.Source) || c.Paths.Source != filepath.Join(filepath.Dir(file), "content") || c.Extra["menu"] == nil {
		t.Fatalf("paths/extra: %+v", c)
	}
	if err := c.Validate("build", filepath.Join(filepath.Dir(file), "out")); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate("dev", filepath.Join(filepath.Dir(file), "preview")); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate("build", c.Paths.Source); err == nil {
		t.Fatal("accepted overlapping output")
	}
	if err := c.Validate("build", ""); err == nil {
		t.Fatal("accepted absent output")
	}
	if err := c.Validate("publish", ""); err == nil {
		t.Fatal("accepted absent release paths")
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, data, field string }{
		{"unknown", "site:\n  titel: bad\n", "titel"},
		{"duplicate", "site:\n  title: A\n  title: B\n", "title"},
		{"coerced string", "site:\n  title: 12\n", "title"},
		{"null", "site: null\n", "site"},
		{"extra type", "extra: []\n", "extra"},
		{"timezone", "site:\n  timezone: Mars/Olympus\n", "timezone"},
		{"subpath", "site:\n  base_url: https://example.com/blog\n", "base_url"},
		{"credentials", "site:\n  base_url: https://u:p@example.com\n", "base_url"},
		{"query", "site:\n  base_url: https://example.com?\n", "base_url"},
		{"fragment", "site:\n  base_url: https://example.com/#\n", "base_url"},
		{"bad port", "site:\n  base_url: https://example.com:99999\n", "base_url"},
		{"listener", "stats:\n  listen_addr: localhost:8787\n", "listen_addr"},
		{"nonloopback", "stats:\n  listen_addr: 0.0.0.0:8787\n", "listen_addr"},
		{"negative", "build:\n  page_size: -1\n", "page_size"},
		{"zero", "stats:\n  report_interval: 0s\n", "report_interval"},
		{"multiple docs", "site: {}\n---\nsite: {}\n", "document"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(file, []byte(tc.data), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Load(file)
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("want %s error, got %v", tc.field, err)
			}
		})
	}
}

func TestStatsDoesNotReadSources(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"manifest.json", "stats.db", "key"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	file := filepath.Join(dir, "config.yaml")
	data := "site:\n  base_url: https://example.com\npaths:\n  source: missing\n  templates: missing-templates\nstats:\n  manifest_path: manifest.json\n  database_path: stats.db\n  visitor_key_path: key\n  public_output: public/stats.json\n  private_output: private/report.html\n"
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"stats", "report"} {
		if err := c.Validate(command, ""); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}
}

func TestInputOutputAndPrivatePathSeparation(t *testing.T) {
	for _, tc := range []struct {
		name, extra string
		output      func(*Config) string
	}{
		{"templates output", "", func(c *Config) string { return c.Paths.Templates }},
		{"output ancestor", "", func(c *Config) string { return filepath.Dir(c.Paths.Source) }},
		{"private in public", "stats:\n  private_output: out/public/report.html\n", func(c *Config) string { return filepath.Join(filepath.Dir(c.Paths.Source), "out") }},
		{"key in static", "stats:\n  visitor_key_path: static/key\n", func(c *Config) string { return filepath.Join(filepath.Dir(c.Paths.Source), "out") }},
		{"database in public stats", "stats:\n  database_path: stats-public/stats.db\n  public_output: stats-public/stats.json\n", func(c *Config) string { return filepath.Join(filepath.Dir(c.Paths.Source), "out") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, err := Load(fixture(t, tc.extra))
			if err == nil {
				err = c.Validate("build", tc.output(c))
			}
			if err == nil {
				t.Fatal("accepted dangerous paths")
			}
		})
	}
}
