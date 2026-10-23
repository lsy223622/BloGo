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
	if err := c.Validate("publish", ""); err == nil {
		t.Fatal("accepted absent release paths")
	}
}

func TestSitePresentationSettings(t *testing.T) {
	file := fixture(t, "build:\n  page_size: 7\n")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "title: 测试", "title: 测试\n  subtitle: 记录与分享", 1))
	if err = os.WriteFile(file, data, 0600); err != nil {
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
	if err := os.WriteFile(file, []byte("site:\n  base_url: https://example.com\n"), 0600); err != nil {
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
