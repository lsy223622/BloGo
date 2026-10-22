package config

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/lsy223622/BloGo/internal/yamlutil"
	"go.yaml.in/yaml/v3"
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	if n.Tag != "!!str" {
		return yamlutil.Error(n, "duration", "expected duration string")
	}
	v, err := time.ParseDuration(n.Value)
	if err != nil {
		return yamlutil.Error(n, "duration", err.Error())
	}
	*d = Duration(v)
	return nil
}

type Config struct {
	Site  Site           `yaml:"site"`
	Paths Paths          `yaml:"paths"`
	Build Build          `yaml:"build"`
	Stats Stats          `yaml:"stats"`
	Extra map[string]any `yaml:"extra"`
	file  string
}
type Site struct {
	BaseURL  string `yaml:"base_url"`
	Title    string `yaml:"title"`
	Timezone string `yaml:"timezone"`
	Language string `yaml:"language"`
}
type Paths struct {
	Source     string `yaml:"source"`
	Templates  string `yaml:"templates"`
	Static     string `yaml:"static"`
	Taxonomies string `yaml:"taxonomies"`
	Releases   string `yaml:"releases"`
	Staging    string `yaml:"staging"`
	Current    string `yaml:"current"`
}
type Build struct {
	ScanInterval    Duration `yaml:"scan_interval"`
	SummaryMaxChars int      `yaml:"summary_max_chars"`
	PageSize        int      `yaml:"page_size"`
	StrictLinks     bool     `yaml:"strict_links"`
	KeepReleases    int      `yaml:"keep_releases"`
}
type Stats struct {
	ListenAddr       string   `yaml:"listen_addr"`
	ManifestPath     string   `yaml:"manifest_path"`
	DatabasePath     string   `yaml:"database_path"`
	VisitorKeyPath   string   `yaml:"visitor_key_path"`
	PublicOutput     string   `yaml:"public_output"`
	PrivateOutput    string   `yaml:"private_output"`
	EventRetention   Duration `yaml:"event_retention"`
	VisitorRetention Duration `yaml:"visitor_retention"`
	ReportInterval   Duration `yaml:"report_interval"`
}

func Load(file string) (*Config, error) {
	file, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	c := &Config{Site: Site{Timezone: "Asia/Shanghai", Language: "zh-CN"}, Paths: Paths{Taxonomies: "taxonomies.yaml"}, Build: Build{ScanInterval: Duration(time.Minute), SummaryMaxChars: 160, PageSize: 10, KeepReleases: 3}, Stats: Stats{ListenAddr: "127.0.0.1:8787", EventRetention: Duration(72 * time.Hour), VisitorRetention: Duration(7 * 24 * time.Hour), ReportInterval: Duration(5 * time.Minute)}, file: file}
	n, err := yamlutil.Decode(data, c)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	fail := func(section, field, msg string) (*Config, error) {
		return nil, fmt.Errorf("%s: %w", file, yamlutil.Error(yamlutil.Field(yamlutil.Field(n, section), field), section+"."+field, msg))
	}
	if _, err := time.LoadLocation(c.Site.Timezone); err != nil || c.Site.Timezone == "" || c.Site.Timezone == "Local" {
		return fail("site", "timezone", "expected IANA timezone")
	}
	if c.Site.BaseURL != "" {
		if err := origin(c.Site.BaseURL); err != nil {
			return fail("site", "base_url", err.Error())
		}
	}
	if err := Listener(c.Stats.ListenAddr); err != nil {
		return fail("stats", "listen_addr", err.Error())
	}
	for _, v := range []struct {
		name  string
		value int
	}{{"summary_max_chars", c.Build.SummaryMaxChars}, {"page_size", c.Build.PageSize}, {"keep_releases", c.Build.KeepReleases}} {
		if v.value <= 0 {
			return fail("build", v.name, "must be positive")
		}
	}
	for _, v := range []struct {
		section, name string
		value         Duration
	}{{"build", "scan_interval", c.Build.ScanInterval}, {"stats", "event_retention", c.Stats.EventRetention}, {"stats", "visitor_retention", c.Stats.VisitorRetention}, {"stats", "report_interval", c.Stats.ReportInterval}} {
		if v.value <= 0 {
			return fail(v.section, v.name, "must be positive")
		}
	}
	for _, p := range []*string{&c.Paths.Source, &c.Paths.Templates, &c.Paths.Static, &c.Paths.Taxonomies, &c.Paths.Releases, &c.Paths.Staging, &c.Paths.Current, &c.Stats.ManifestPath, &c.Stats.DatabasePath, &c.Stats.VisitorKeyPath, &c.Stats.PublicOutput, &c.Stats.PrivateOutput} {
		if *p != "" {
			if !filepath.IsAbs(*p) {
				*p = filepath.Join(filepath.Dir(file), *p)
			}
			*p = filepath.Clean(*p)
		}
	}
	return c, nil
}

func origin(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid HTTP(S) origin")
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || strings.Contains(raw, "#") || strings.Contains(raw, "\\") {
		return fmt.Errorf("expected HTTP(S) origin without credentials, path, query or fragment")
	}
	if strings.ContainsAny(u.Hostname(), " \t\r\n%") {
		return fmt.Errorf("invalid hostname")
	}
	if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("invalid port")
	}
	if u.Port() != "" {
		port, err := strconv.Atoi(u.Port())
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("invalid port")
		}
	}
	return nil
}

func Listener(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("expected loopback IP and port")
	}
	ip := net.ParseIP(host)
	p, e := strconv.Atoi(port)
	if ip == nil || !ip.IsLoopback() || e != nil || p < 1 || p > 65535 {
		return fmt.Errorf("expected loopback IP and port 1..65535")
	}
	return nil
}

func (c *Config) Validate(command, output string) error {
	fail := func(field, msg string) error { return fmt.Errorf("%s: %s: %s", c.file, field, msg) }
	if c.Site.BaseURL == "" {
		return fail("site.base_url", "required")
	}
	switch command {
	case "build", "dev", "publish", "watch":
		if strings.TrimSpace(c.Site.Title) == "" {
			return fail("site.title", "required")
		}
		for _, v := range []struct{ name, path string }{{"source", c.Paths.Source}, {"templates", c.Paths.Templates}, {"static", c.Paths.Static}} {
			if err := readable(v.path, true); err != nil {
				return fail("paths."+v.name, err.Error())
			}
		}
		if err := readable(c.Paths.Taxonomies, false); err != nil {
			return fail("paths.taxonomies", err.Error())
		}
		if command == "build" || command == "dev" {
			if output == "" {
				return fail("output", "required")
			}
		}
		if command == "publish" || command == "watch" {
			for _, v := range []struct{ name, path string }{{"releases", c.Paths.Releases}, {"staging", c.Paths.Staging}, {"current", c.Paths.Current}} {
				if v.path == "" {
					return fail("paths."+v.name, "required")
				}
			}
			if err := sameFilesystem(c.Paths.Staging, c.Paths.Releases, filepath.Dir(c.Paths.Current)); err != nil {
				return fail("paths", err.Error())
			}
		}
	case "stats", "report":
		if err := readable(c.Stats.ManifestPath, false); err != nil {
			return fail("stats.manifest_path", err.Error())
		}
		if c.Stats.DatabasePath == "" {
			return fail("stats.database_path", "required")
		}
		if command == "stats" {
			if err := readable(c.Stats.VisitorKeyPath, false); err != nil {
				return fail("stats.visitor_key_path", err.Error())
			}
			if err := privateKey(c.Stats.VisitorKeyPath); err != nil {
				return fail("stats.visitor_key_path", err.Error())
			}
		}
		if command == "report" {
			if err := readable(c.Stats.DatabasePath, false); err != nil {
				return fail("stats.database_path", err.Error())
			}
			if c.Stats.PublicOutput == "" || c.Stats.PrivateOutput == "" {
				return fail("stats", "public_output and private_output required")
			}
		}
	default:
		return fail("command", "unknown command")
	}
	if output != "" && !filepath.IsAbs(output) {
		output = filepath.Join(filepath.Dir(c.file), output)
	}
	if err := c.validatePaths(command, output); err != nil {
		return fail("paths", err.Error())
	}
	type targetPath struct {
		path      string
		directory bool
	}
	var targets []targetPath
	switch command {
	case "build", "dev":
		targets = append(targets, targetPath{output, true})
	case "publish", "watch":
		for _, p := range []string{c.Paths.Staging, c.Paths.Releases, filepath.Dir(c.Paths.Current)} {
			targets = append(targets, targetPath{p, true})
		}
	case "stats":
		targets = append(targets, targetPath{c.Stats.DatabasePath, false})
	case "report":
		for _, p := range []string{c.Stats.PublicOutput, c.Stats.PrivateOutput} {
			targets = append(targets, targetPath{p, false})
		}
	}
	for _, target := range targets {
		if err := writableTarget(target.path, target.directory); err != nil {
			return fail("output", err.Error())
		}
	}
	return nil
}

func readable(path string, dir bool) error {
	if path == "" {
		return fmt.Errorf("required")
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.IsDir() != dir || (!dir && !info.Mode().IsRegular()) {
		return fmt.Errorf("unexpected file type: %s", path)
	}
	if dir {
		_, err = f.ReadDir(1)
		if err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
