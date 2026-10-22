//go:build linux

package integration

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/stats"
	"go.yaml.in/yaml/v3"
)

func TestPermissionProbe(t *testing.T) {
	if os.Getenv("BLOGO_PROBE") == "" {
		t.Skip("subprocess helper")
	}
	var denied []string
	if err := json.Unmarshal([]byte(os.Getenv("BLOGO_PROBE")), &denied); err != nil {
		t.Fatal(err)
	}
	for _, path := range denied {
		if _, err := os.ReadFile(path); !os.IsPermission(err) {
			t.Fatalf("uid %d must be denied %s: %v", os.Getuid(), path, err)
		}
	}
}

func TestDeployedLinux(t *testing.T) {
	if os.Getenv("BLOGO_DEPLOY_TEST") != "1" {
		t.Skip("run scripts/verify-stack.ps1 in its dedicated container")
	}
	if os.Geteuid() != 0 {
		t.Fatal("test setup requires container root; application processes run as separate users")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(repo, ".superpowers/linux/blog")
	root, err := os.MkdirTemp("/tmp", "blogo-stack-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	const meta, dbgroup, web, sourceGroup = 21000, 21001, 21002, 21010
	const builder, collector, reporter = 21003, 21004, 21005
	must(os.Chown(root, builder, meta))
	must(os.Chmod(root, 0755))
	mkdir := func(path string, mode os.FileMode, uid, gid int) {
		t.Helper()
		must(os.MkdirAll(path, mode))
		must(os.Chown(path, uid, gid))
		must(os.Chmod(path, mode))
	}
	source := filepath.Join(root, "source")
	mkdir(source, 0750, builder, sourceGroup)
	for _, name := range []string{"content", "templates", "static"} {
		must(os.CopyFS(filepath.Join(source, name), os.DirFS(filepath.Join(repo, name))))
	}
	taxonomy, err := os.ReadFile(filepath.Join(repo, "taxonomies.yaml"))
	must(err)
	must(os.WriteFile(filepath.Join(source, "taxonomies.yaml"), taxonomy, 0640))
	must(filepath.WalkDir(source, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err = os.Chown(path, builder, sourceGroup); err != nil {
			return err
		}
		mode := os.FileMode(0640)
		if d.IsDir() {
			mode = 0750
		}
		return os.Chmod(path, mode)
	}))
	for _, dir := range []string{"releases", "staging"} {
		mkdir(filepath.Join(root, dir), 0755, builder, meta)
	}
	dataDir := filepath.Join(root, "stats")
	mkdir(dataDir, 0750, collector, dbgroup)
	mkdir(filepath.Join(dataDir, "database"), os.ModeSetgid|0770, collector, dbgroup)
	mkdir(filepath.Join(root, "stats-public"), os.ModeSetgid|0750, reporter, web)
	mkdir(filepath.Join(root, "stats-private"), 0700, reporter, reporter)
	mkdir(filepath.Join(root, "git-home"), 0700, 0, 0)
	credentials := filepath.Join(root, "git-home/credentials")
	must(os.WriteFile(credentials, []byte("fixture credentials"), 0600))
	c := config.Config{Site: config.Site{BaseURL: "http://127.0.0.1:8080", Title: "Isolated stack", Timezone: "Asia/Shanghai", Language: "zh-CN"}, Paths: config.Paths{Source: filepath.Join(source, "content"), Templates: filepath.Join(source, "templates"), Static: filepath.Join(source, "static"), Taxonomies: filepath.Join(source, "taxonomies.yaml"), Releases: filepath.Join(root, "releases"), Staging: filepath.Join(root, "staging"), Current: filepath.Join(root, "current")}, Stats: config.Stats{ListenAddr: "127.0.0.1:8787", ManifestPath: filepath.Join(root, "current/meta/manifest.json"), DatabasePath: filepath.Join(dataDir, "database/stats.db"), VisitorKeyPath: filepath.Join(dataDir, "visitor.key"), PublicOutput: filepath.Join(root, "stats-public/stats.json"), PrivateOutput: filepath.Join(root, "stats-private/report.html")}}
	configPath := filepath.Join(root, "config.yaml")
	encoded, err := yaml.Marshal(c)
	must(err)
	// Leave defaults to config.Load rather than serializing zero durations.
	var configMap map[string]any
	must(yaml.Unmarshal(encoded, &configMap))
	delete(configMap, "build")
	for _, key := range []string{"event_retention", "visitor_retention", "report_interval"} {
		delete(configMap["stats"].(map[string]any), key)
	}
	encoded, err = yaml.Marshal(configMap)
	must(err)
	must(os.WriteFile(configPath, encoded, 0644))
	command := func(uid, gid int, groups []uint32, args ...string) *exec.Cmd {
		cmd := exec.Command(binary, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups}}
		return cmd
	}
	run := func(uid, gid int, groups []uint32, args ...string) {
		t.Helper()
		cmd := command(uid, gid, groups, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
	}
	unit, err := os.ReadFile(filepath.Join(repo, "deploy/blog-watch.service"))
	must(err)
	var builderMask int64 = -1
	for _, line := range strings.Split(string(unit), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "UMask="); ok {
			builderMask, err = strconv.ParseInt(value, 8, 32)
			must(err)
		}
	}
	if builderMask < 0 {
		t.Fatal("watch service umask missing")
	}
	publish := func() {
		t.Helper()
		old := syscall.Umask(int(builderMask))
		defer syscall.Umask(old)
		run(builder, meta, []uint32{sourceGroup}, "publish", "--config", configPath)
	}
	publish()
	probe := func(uid, gid int, groups []uint32, paths ...string) {
		t.Helper()
		payload, _ := json.Marshal(paths)
		cmd := exec.Command(os.Args[0], "-test.run=^TestPermissionProbe$")
		cmd.Env = append(os.Environ(), "BLOGO_PROBE="+string(payload))
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid), Groups: groups}}
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("permission probe: %v\n%s", err, out)
		}
	}
	var collectorProcess *exec.Cmd
	var collectorLog bytes.Buffer
	startCollector := func() {
		t.Helper()
		collectorLog.Reset()
		old := syscall.Umask(0007)
		collectorProcess = command(collector, dbgroup, []uint32{meta}, "stats", "--config", configPath)
		collectorProcess.Stdout = &collectorLog
		collectorProcess.Stderr = &collectorLog
		err := collectorProcess.Start()
		syscall.Umask(old)
		must(err)
	}
	stopCollector := func() {
		t.Helper()
		if collectorProcess != nil {
			must(collectorProcess.Process.Signal(syscall.SIGTERM))
			err := collectorProcess.Wait()
			collectorProcess = nil
			if err != nil {
				t.Fatalf("collector: %v %s", err, collectorLog.String())
			}
		}
	}
	defer func() {
		if collectorProcess != nil {
			collectorProcess.Process.Kill()
			collectorProcess.Wait()
		}
	}()
	startCollector()
	client := &http.Client{Timeout: 3 * time.Second}
	wait := func(url string, code int) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			response, err := client.Get(url)
			if err == nil {
				response.Body.Close()
				if response.StatusCode == code {
					return
				}
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatalf("not ready %s; %s", url, collectorLog.String())
	}
	wait("http://127.0.0.1:8787/api/events", 405)
	key, err := os.ReadFile(c.Stats.VisitorKeyPath)
	must(err)
	for _, path := range []string{c.Stats.DatabasePath, c.Stats.DatabasePath + "-wal", c.Stats.DatabasePath + "-shm"} {
		info, err := os.Stat(path)
		must(err)
		if info.Mode().Perm() != 0660 {
			t.Fatalf("shared SQLite mode %s: %o", path, info.Mode().Perm())
		}
	}
	probe(collector, dbgroup, []uint32{meta}, filepath.Join(source, "content/posts/welcome.md"), credentials)
	probe(reporter, dbgroup, []uint32{meta, web}, c.Stats.VisitorKeyPath, filepath.Join(source, "content/posts/welcome.md"), credentials)
	nginxConfig, err := os.ReadFile(filepath.Join(repo, "deploy/nginx.conf"))
	must(err)
	nginxConfig = []byte(strings.NewReplacer("/srv/blog/current", c.Paths.Current, "/srv/blog-stats-public", filepath.Dir(c.Stats.PublicOutput), "/etc/blog/redirects.conf", filepath.Join(root, "redirects.conf")).Replace(string(nginxConfig)))
	nginxPath := filepath.Join(root, "nginx.conf")
	must(os.WriteFile(nginxPath, nginxConfig, 0644))
	must(os.WriteFile(filepath.Join(root, "redirects.conf"), nil, 0644))
	nginx := exec.Command("nginx", "-c", nginxPath, "-g", "daemon off;")
	nginx.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 101, Gid: 101, Groups: []uint32{web}}}
	var nginxLog bytes.Buffer
	nginx.Stdout = &nginxLog
	nginx.Stderr = &nginxLog
	must(nginx.Start())
	defer func() { nginx.Process.Signal(syscall.SIGTERM); nginx.Wait() }()
	wait("http://127.0.0.1:8080/", 200)
	response, err := client.Get("http://127.0.0.1:8080/")
	must(err)
	firstHTML, err := io.ReadAll(response.Body)
	response.Body.Close()
	must(err)
	oldCSS := regexp.MustCompile(`/assets/theme\.[0-9a-f]{16}\.css`).FindString(string(firstHTML))
	if oldCSS == "" {
		t.Fatal("old CSS not found")
	}
	cssPath := filepath.Join(c.Paths.Static, "assets/theme.css")
	css, err := os.ReadFile(cssPath)
	must(err)
	must(os.WriteFile(cssPath, append(css, []byte("\nbody{letter-spacing:.001px}")...), 0640))
	publish()
	response, err = client.Get("http://127.0.0.1:8080" + oldCSS)
	must(err)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatalf("in-flight old HTML asset: %d", response.StatusCode)
	}
	send := func(id, page string, code int) {
		t.Helper()
		payload := fmt.Sprintf(`{"v":1,"event_id":%q,"page_id":%q,"visitor_id":%q,"referrer_host":"Search.Example"}`, strings.Repeat(id, 32), page, strings.Repeat("f", 32))
		response, err := client.Post("http://127.0.0.1:8080/api/events", "application/json", strings.NewReader(payload))
		must(err)
		defer response.Body.Close()
		body, _ := io.ReadAll(response.Body)
		if response.StatusCode != code {
			t.Fatalf("event %s: %d %s", page, response.StatusCode, body)
		}
	}
	send("a", "post:welcome", 204)
	send("a", "post:welcome", 204)
	send("b", "home", 204)
	send("c", "not-found", 204)
	report := func() { t.Helper(); run(reporter, dbgroup, []uint32{meta, web}, "report", "--config", configPath) }
	report()
	probe(101, 101, []uint32{web}, c.Stats.DatabasePath, c.Stats.PrivateOutput, c.Stats.VisitorKeyPath, c.Stats.ManifestPath, credentials)
	response, err = client.Get("http://127.0.0.1:8080/data/stats.json")
	must(err)
	data, err := io.ReadAll(response.Body)
	response.Body.Close()
	must(err)
	if response.StatusCode != 200 || response.Header.Get("Cache-Control") != "no-cache" {
		t.Fatal("public JSON cache/status")
	}
	var public stats.PublicReport
	must(json.Unmarshal(data, &public))
	if public.BuildID == "" || public.Pages["post:welcome"].Views != 1 {
		t.Fatal("public report counts")
	}
	for _, url := range []string{"/meta/manifest.json", "/stats.db", "/data/private/report.html", "/.git/config", "/source/content/posts/welcome.md", "/data/../stats.db", "/data/%2e%2e/stats.db"} {
		response, err := client.Get("http://127.0.0.1:8080" + url)
		must(err)
		response.Body.Close()
		if response.StatusCode != 404 {
			t.Fatalf("private HTTP %s: %d", url, response.StatusCode)
		}
	}
	stopCollector()
	wait("http://127.0.0.1:8080/", 200)
	send("d", "post:welcome", 502)
	report()
	startCollector()
	wait("http://127.0.0.1:8787/api/events", 405)
	send("a", "post:welcome", 204)
	send("d", "post:welcome", 204)
	report()
	currentKey, err := os.ReadFile(c.Stats.VisitorKeyPath)
	must(err)
	if !bytes.Equal(key, currentKey) {
		t.Fatal("key changed on restart")
	}
	backup := filepath.Join(root, "snapshot.db")
	database, err := sql.Open("sqlite", c.Stats.DatabasePath)
	must(err)
	_, err = database.ExecContext(context.Background(), "VACUUM INTO ?", backup)
	must(err)
	must(database.Close())
	must(os.WriteFile(filepath.Join(root, "key.backup"), key, 0600))
	stopCollector()
	must(os.Rename(c.Stats.DatabasePath, filepath.Join(root, "old.db")))
	must(os.Rename(backup, c.Stats.DatabasePath))
	must(os.Chown(c.Stats.DatabasePath, collector, dbgroup))
	must(os.Chmod(c.Stats.DatabasePath, 0660))
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(c.Stats.DatabasePath + suffix); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
	}
	savedKey, err := os.ReadFile(filepath.Join(root, "key.backup"))
	must(err)
	must(os.WriteFile(c.Stats.VisitorKeyPath, savedKey, 0600))
	startCollector()
	wait("http://127.0.0.1:8787/api/events", 405)
	send("a", "post:welcome", 204)
	send("e", "post:welcome", 204)
	report()
	data, err = os.ReadFile(c.Stats.PublicOutput)
	must(err)
	must(json.Unmarshal(data, &public))
	if public.Pages["post:welcome"].Views != 3 {
		t.Fatalf("restore counts: %s", data)
	}
	before, err := os.ReadFile(c.Stats.PublicOutput)
	must(err)
	manifest, err := os.ReadFile(c.Stats.ManifestPath)
	must(err)
	must(os.WriteFile(c.Stats.ManifestPath, []byte("broken"), 0640))
	failed := command(reporter, dbgroup, []uint32{meta, web}, "report", "--config", configPath)
	if err = failed.Run(); err == nil {
		t.Fatal("bad manifest report succeeded")
	}
	after, err := os.ReadFile(c.Stats.PublicOutput)
	must(err)
	if !bytes.Equal(before, after) {
		t.Fatal("failed timer removed old report")
	}
	must(os.WriteFile(c.Stats.ManifestPath, manifest, 0640))
	report()
	stopCollector()
	reader, err := stats.OpenReader(context.Background(), c.Stats.DatabasePath)
	must(err)
	must(reader.Close())
	response, err = client.Post("http://127.0.0.1:8080/api/events", "application/json", strings.NewReader(strings.Repeat("x", 2049)))
	must(err)
	response.Body.Close()
	if response.StatusCode != 413 {
		t.Fatalf("Nginx body limit: %d", response.StatusCode)
	}
	limited := 0
	for n := 0; n < 45; n++ {
		request, err := http.NewRequest("POST", "http://127.0.0.1:8080/api/events", strings.NewReader("{}"))
		must(err)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Forwarded-For", fmt.Sprintf("192.0.2.%d", n))
		response, err = client.Do(request)
		must(err)
		response.Body.Close()
		if response.StatusCode == 429 {
			limited++
		}
	}
	if limited == 0 {
		t.Fatal("Nginx connection-address rate limit did not reject the burst")
	}
	t.Log("Passed: separate nonroot builder/collector/reporter/nginx, private file denials, DB/WAL/SHM 0660, loopback API, Nginx cache/private URLs, collector stop/restart, VACUUM INTO + key restore, report failure preservation/recovery")
	if path := os.Getenv("BLOGO_TEST_RESULT"); path != "" {
		must(os.WriteFile(path, []byte(`{"passed":true,"nginx_uid":101,"collector_uid":21004,"reporter_uid":21005,"backup_restore":true,"private_file_denials":true}`+"\n"), 0644))
	}
}
