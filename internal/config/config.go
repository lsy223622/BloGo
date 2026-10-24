package config

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/netip"
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
	Site     Site             `yaml:"-"`
	Server   Server           `yaml:"server"`
	Timezone string           `yaml:"timezone"`
	Content  *ContentSettings `yaml:"-"`
	Paths    Paths            `yaml:"paths"`
	Build    Build            `yaml:"build"`
	Stats    Stats            `yaml:"stats"`
	Extra    map[string]any   `yaml:"-"`
	file     string
	digest   [32]byte
}

func (c *Config) FilePath() string     { return c.file }
func (c *Config) FileDigest() [32]byte { return c.digest }

type Server struct {
	ListenAddr     string   `yaml:"listen_addr"`
	BaseURL        string   `yaml:"base_url"`
	TrustedProxies []string `yaml:"trusted_proxies"`
}

type Site struct {
	BaseURL  string `yaml:"base_url"`
	Title    string `yaml:"title"`
	Subtitle string `yaml:"subtitle"`
	Timezone string `yaml:"timezone"`
	Language string `yaml:"language"`
}
type Paths struct {
	Source     string `yaml:"content"`
	Data       string `yaml:"data"`
	Assets     string `yaml:"-"`
	Templates  string `yaml:"templates"`
	Static     string `yaml:"static"`
	Taxonomies string `yaml:"-"`
	Releases   string `yaml:"-"`
	Staging    string `yaml:"-"`
	Current    string `yaml:"-"`
}
type Build struct {
	ScanInterval    Duration `yaml:"scan_interval"`
	SummaryMaxChars int      `yaml:"-"`
	PageSize        int      `yaml:"-"`
	StrictLinks     bool     `yaml:"strict_links"`
	KeepReleases    int      `yaml:"keep_releases"`
}
type Details struct {
	Enabled   bool     `yaml:"enabled"`
	Retention Duration `yaml:"retention"`
}
type Stats struct {
	Enabled          bool     `yaml:"enabled"`
	Details          Details  `yaml:"details"`
	ListenAddr       string   `yaml:"-"`
	ManifestPath     string   `yaml:"-"`
	DatabasePath     string   `yaml:"-"`
	VisitorKeyPath   string   `yaml:"-"`
	PublicOutput     string   `yaml:"-"`
	PrivateOutput    string   `yaml:"-"`
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
	c := &Config{Server: Server{ListenAddr: "127.0.0.1:25646", BaseURL: "http://127.0.0.1:25646", TrustedProxies: []string{"127.0.0.1", "::1"}}, Timezone: "Asia/Shanghai", Paths: Paths{Source: "content", Templates: "templates", Static: "static", Data: "data"}, Build: Build{ScanInterval: Duration(time.Minute), KeepReleases: 3}, Stats: Stats{Enabled: true, Details: Details{Retention: Duration(168 * time.Hour)}, EventRetention: Duration(72 * time.Hour), VisitorRetention: Duration(168 * time.Hour), ReportInterval: Duration(5 * time.Minute)}, file: file}
	if _, err = yamlutil.Decode(data, c); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	c.digest = sha256.Sum256(data)
	if _, err = time.LoadLocation(c.Timezone); err != nil || c.Timezone == "" || c.Timezone == "Local" {
		return nil, fmt.Errorf("%s: timezone: expected IANA timezone", file)
	}
	if err = origin(c.Server.BaseURL); err != nil {
		return nil, fmt.Errorf("%s: server.base_url: %w", file, err)
	}
	if err = Listener(c.Server.ListenAddr); err != nil {
		return nil, fmt.Errorf("%s: server.listen_addr: %w", file, err)
	}
	for _, proxy := range c.Server.TrustedProxies {
		if _, e := netip.ParsePrefix(proxy); e != nil {
			address, err := netip.ParseAddr(proxy)
			if err != nil || address.Zone() != "" {
				return nil, fmt.Errorf("%s: trusted_proxies: invalid IP or prefix", file)
			}
		}
	}
	if c.Build.KeepReleases < 1 {
		return nil, fmt.Errorf("%s: keep_releases must be positive", file)
	}
	for _, interval := range []Duration{c.Build.ScanInterval, c.Stats.EventRetention, c.Stats.VisitorRetention, c.Stats.ReportInterval, c.Stats.Details.Retention} {
		if interval <= 0 {
			return nil, fmt.Errorf("%s: intervals and retention must be positive", file)
		}
	}
	for _, path := range []*string{&c.Paths.Source, &c.Paths.Templates, &c.Paths.Static, &c.Paths.Data} {
		if *path == "" {
			return nil, fmt.Errorf("%s: paths must not be empty", file)
		}
		if !filepath.IsAbs(*path) {
			*path = filepath.Join(filepath.Dir(file), *path)
		}
		*path = filepath.Clean(*path)
	}
	for _, input := range []string{c.Paths.Source, c.Paths.Templates, c.Paths.Static, file} {
		if contains(c.Paths.Data, input) || contains(input, c.Paths.Data) {
			return nil, fmt.Errorf("%s: data overlaps input", file)
		}
	}
	c.Paths.Taxonomies = filepath.Join(c.Paths.Source, "site.yaml")
	c.Paths.Assets = filepath.Join(c.Paths.Source, "assets")
	c.Paths.Releases = filepath.Join(c.Paths.Data, "releases")
	c.Paths.Staging = filepath.Join(c.Paths.Data, "staging")
	c.Paths.Current = filepath.Join(c.Paths.Data, "current")
	c.Stats.ManifestPath = filepath.Join(c.Paths.Current, "meta/manifest.json")
	c.Stats.DatabasePath = filepath.Join(c.Paths.Data, "statistics/stats.db")
	c.Stats.VisitorKeyPath = filepath.Join(c.Paths.Data, "statistics/visitor.key")
	c.Stats.PublicOutput = filepath.Join(c.Paths.Data, "public/stats.json")
	c.Stats.PrivateOutput = filepath.Join(c.Paths.Data, "private/report.html")
	c.Stats.ListenAddr = c.Server.ListenAddr
	c.Site.BaseURL = c.Server.BaseURL
	c.Site.Timezone = c.Timezone
	settings, e := LoadContentSettings(c.Paths.Taxonomies)
	if e == nil {
		c.ApplyContentSettings(settings)
	} else if !os.IsNotExist(e) {
		return nil, e
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
		if c.Paths.Data != "" {
			data, err := resolved(c.Paths.Data)
			if err != nil {
				return fail("paths.data", err.Error())
			}
			for _, input := range []string{c.Paths.Source, c.Paths.Templates, c.Paths.Static, c.file} {
				if input == "" {
					continue
				}
				path, err := resolved(input)
				if err != nil {
					return err
				}
				if contains(data, path) || contains(path, data) {
					return fail("paths.data", "overlaps input")
				}
			}
		}
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
			if c.Stats.VisitorKeyPath == "" {
				return fail("stats.visitor_key_path", "required")
			}
			if err := readable(c.Stats.VisitorKeyPath, false); err != nil {
				if !os.IsNotExist(err) {
					return fail("stats.visitor_key_path", err.Error())
				}
				if err = writableTarget(c.Stats.VisitorKeyPath, false); err != nil {
					return fail("stats.visitor_key_path", err.Error())
				}
			} else if err = privateKey(c.Stats.VisitorKeyPath); err != nil {
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
