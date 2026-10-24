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
	if err := os.WriteFile(filepath.Join(dir, "content/site.yaml"), []byte("schema_version: 1\ntitle: 测试\n"), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	data := "server:\n  base_url: https://example.com\n" + extra
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestLoadRejectsScopedTrustedProxy(t *testing.T) {
	file := fixture(t, "")
	if err := os.WriteFile(file, []byte("server:\n  trusted_proxies: [\"fe80::1%eth0\"]\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(file); err == nil {
		t.Fatal("scoped proxy accepted")
	}
}

func TestLoadDefaultsAndCommandBoundaries(t *testing.T) {
	file := fixture(t, "")
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Site.Timezone != "Asia/Shanghai" || c.Site.Language != "zh-CN" || c.Build.PageSize != 10 || c.Build.SummaryMaxChars != 160 || c.Build.KeepReleases != 3 || time.Duration(c.Build.ScanInterval) != time.Minute {
		t.Fatalf("defaults: %+v", c)
	}
	if !filepath.IsAbs(c.Paths.Source) || c.Paths.Source != filepath.Join(filepath.Dir(file), "content") {
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
	if err := c.Validate("build", file); err == nil {
		t.Fatal("accepted file as output directory")
	}
	ordinaryFile := filepath.Join(filepath.Dir(file), "ordinary-file")
	if err := os.WriteFile(ordinaryFile, []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate("build", ordinaryFile); err == nil {
		t.Fatal("accepted unrelated ordinary file as output directory")
	}

}

func TestSitePresentationSettings(t *testing.T) {
	file := fixture(t, "")
	settings := filepath.Join(filepath.Dir(file), "content/site.yaml")
	err := os.WriteFile(settings, []byte("schema_version: 1\ntitle: 测试\nsubtitle: 记录与分享\npage_size: 7\n"), 0600)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if c.Site.Title != "测试" || c.Site.Subtitle != "记录与分享" || c.Build.PageSize != 7 {
		t.Fatalf("presentation settings: %+v %+v", c.Site, c.Build)
	}
}

func TestPublishingPathRelations(t *testing.T) {
	c, err := Load(fixture(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	base := filepath.Dir(c.Paths.Source)
	c.Paths.Staging = filepath.Join(base, "staging")
	c.Paths.Releases = filepath.Join(base, "releases")
	c.Paths.Current = filepath.Join(base, "current")
	if err = c.Validate("publish", ""); err != nil {
		t.Fatal(err)
	}
	c.Paths.Staging = filepath.Join(c.Paths.Releases, "staging")
	if err = c.Validate("publish", ""); err == nil {
		t.Fatal("accepted nested staging/releases")
	}
	c.Paths.Staging = filepath.Join(base, "staging")
	for _, current := range []string{c.Paths.Releases, filepath.Dir(c.Paths.Releases)} {
		c.Paths.Current = current
		if err = c.Validate("publish", ""); err == nil {
			t.Fatalf("accepted dangerous current %s", current)
		}
	}
	c.Paths.Current = filepath.Join(c.Paths.Releases, "ordinary-current")
	if err = os.MkdirAll(c.Paths.Current, 0700); err != nil {
		t.Fatal(err)
	}
	if err = c.Validate("publish", ""); err == nil {
		t.Fatal("accepted ordinary directory as current")
	}
}

func TestManifestCannotBeOverwritten(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifest, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(key, []byte("key"), 0600); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(file, []byte("server:\n  base_url: https://example.com\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ command, field string }{{"stats", "database"}, {"report", "public"}, {"report", "private"}} {
		t.Run(tc.command+tc.field, func(t *testing.T) {
			c, err := Load(file)
			if err != nil {
				t.Fatal(err)
			}
			c.Stats.ManifestPath = manifest
			c.Stats.DatabasePath = filepath.Join(dir, "db")
			c.Stats.VisitorKeyPath = key
			c.Stats.PublicOutput = filepath.Join(dir, "public/stats.json")
			c.Stats.PrivateOutput = filepath.Join(dir, "private/report.html")
			if tc.field == "database" {
				c.Stats.DatabasePath = manifest
			} else {
				if err = os.WriteFile(c.Stats.DatabasePath, []byte("db"), 0600); err != nil {
					t.Fatal(err)
				}
				if tc.field == "public" {
					c.Stats.PublicOutput = manifest
				} else {
					c.Stats.PrivateOutput = manifest
				}
			}
			if err = c.Validate(tc.command, ""); err == nil {
				t.Fatal("accepted Manifest overwrite")
			}
		})
	}
}

func TestInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, data, field string }{
		{"unknown", "server:\n  bas_url: bad\n", "bas_url"},
		{"duplicate", "server:\n  base_url: https://a.example\n  base_url: https://b.example\n", "base_url"},
		{"coerced string", "server:\n  base_url: 12\n", "base_url"},
		{"null", "server: null\n", "server"},
		{"extra type", "extra: []\n", "extra"},
		{"timezone", "timezone: Mars/Olympus\n", "timezone"},
		{"subpath", "server:\n  base_url: https://example.com/blog\n", "base_url"},
		{"credentials", "server:\n  base_url: https://u:p@example.com\n", "base_url"},
		{"query", "server:\n  base_url: https://example.com?\n", "base_url"},
		{"fragment", "server:\n  base_url: https://example.com/#\n", "base_url"},
		{"bad port", "server:\n  base_url: https://example.com:99999\n", "base_url"},
		{"listener", "server:\n  listen_addr: localhost:8787\n", "listen_addr"},
		{"nonloopback", "server:\n  listen_addr: 0.0.0.0:8787\n", "listen_addr"},
		{"negative", "build:\n  keep_releases: -1\n", "keep_releases"},
		{"zero", "stats:\n  report_interval: 0s\n", "intervals"},
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
	data := "server:\n  base_url: https://example.com\npaths:\n  content: missing\n  templates: missing-templates\n"
	if err := os.WriteFile(file, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}
	c.Stats.ManifestPath = filepath.Join(dir, "manifest.json")
	c.Stats.DatabasePath = filepath.Join(dir, "stats.db")
	c.Stats.VisitorKeyPath = filepath.Join(dir, "key")
	c.Stats.PublicOutput = filepath.Join(dir, "public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(dir, "private/report.html")
	for _, command := range []string{"stats", "report"} {
		if err := c.Validate(command, ""); err != nil {
			t.Fatalf("%s: %v", command, err)
		}
	}
}

func TestInputOutputAndPrivatePathSeparation(t *testing.T) {
	for _, test := range []struct {
		name   string
		change func(*Config)
		output func(*Config) string
	}{
		{"templates output", nil, func(c *Config) string { return c.Paths.Templates }},
		{"output ancestor", nil, func(c *Config) string { return filepath.Dir(c.Paths.Source) }},
		{"private in public", func(c *Config) {
			c.Stats.PrivateOutput = filepath.Join(filepath.Dir(c.Paths.Source), "out/public/report.html")
		}, nil},
		{"key in static", func(c *Config) { c.Stats.VisitorKeyPath = filepath.Join(c.Paths.Static, "key") }, nil},
		{"database in public stats", func(c *Config) { c.Stats.DatabasePath = filepath.Join(filepath.Dir(c.Stats.PublicOutput), "stats.db") }, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, err := Load(fixture(t, ""))
			if err != nil {
				t.Fatal(err)
			}
			if test.change != nil {
				test.change(c)
			}
			output := filepath.Join(filepath.Dir(c.Paths.Source), "out")
			if test.output != nil {
				output = test.output(c)
			}
			if err = c.Validate("build", output); err == nil {
				t.Fatal("accepted dangerous paths")
			}
		})
	}
}
