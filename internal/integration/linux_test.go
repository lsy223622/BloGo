//go:build linux

package integration

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/stats"
)

func TestDeployedLinux(t *testing.T) {
	if os.Getenv("BLOGO_DEPLOY_TEST") != "1" {
		t.Skip("run scripts/verify-stack.ps1")
	}
	if os.Geteuid() == 0 {
		t.Fatal("application acceptance must run as an ordinary user")
	}
	repo, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	archive := os.Getenv("BLOGO_ARCHIVE")
	if archive == "" {
		archive = filepath.Join(repo, "output/release/BloGo-v0.1.0-local-linux-amd64.tar.gz")
	}
	root := t.TempDir()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	unpack := func(destination string) string {
		t.Helper()
		must(os.MkdirAll(destination, 0700))
		if data, e := exec.Command("tar", "-xzf", archive, "-C", destination).CombinedOutput(); e != nil {
			t.Fatalf("unpack: %v %s", e, data)
		}
		dirs, e := filepath.Glob(filepath.Join(destination, "BloGo-*"))
		must(e)
		if len(dirs) != 1 {
			t.Fatal("archive root")
		}
		return dirs[0]
	}
	home := unpack(filepath.Join(root, "initial"))
	configFile := filepath.Join(home, "config.yaml")
	config := `server:
  listen_addr: 127.0.0.1:25646
  base_url: http://127.0.0.1:8080
  trusted_proxies: [127.0.0.1]
build:
  scan_interval: 100ms
stats:
  report_interval: 100ms
  details:
    enabled: false
    retention: 168h
`
	must(os.WriteFile(configFile, []byte(config), 0644))
	nginxConfig := fmt.Sprintf(`worker_processes 1;
error_log stderr warn;
pid %s/nginx.pid;
events { worker_connections 64; }
http {
 access_log off;
 client_body_temp_path %s/body;
 proxy_temp_path %s/proxy;
 fastcgi_temp_path %s/fastcgi;
 uwsgi_temp_path %s/uwsgi;
 scgi_temp_path %s/scgi;
 server { listen 127.0.0.1:8080;
  location / {
   proxy_pass http://127.0.0.1:25646;
   proxy_http_version 1.1;
   proxy_set_header Connection "";
   proxy_set_header Host $host;
   proxy_set_header X-Forwarded-Proto $scheme;
   proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  }
 }
}`, root, root, root, root, root, root)
	nginxPath := filepath.Join(root, "nginx.conf")
	must(os.WriteFile(nginxPath, []byte(nginxConfig), 0644))
	nginx := exec.Command("nginx", "-c", nginxPath, "-g", "daemon off;")
	nginxLog, e := os.Create(filepath.Join(root, "nginx.log"))
	must(e)
	defer nginxLog.Close()
	nginx.Stdout = nginxLog
	nginx.Stderr = nginxLog
	must(nginx.Start())
	defer func() { nginx.Process.Signal(syscall.SIGTERM); nginx.Wait() }()
	var process *exec.Cmd
	var processLog *os.File
	start := func() {
		t.Helper()
		processLog, e = os.OpenFile(filepath.Join(root, "service.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		must(e)
		process = exec.Command(filepath.Join(home, "blog"), "serve", "--config", filepath.Join(home, "config.yaml"))
		process.Dir = home
		process.Stdout = processLog
		process.Stderr = processLog
		must(process.Start())
	}
	stop := func() {
		t.Helper()
		if process == nil {
			return
		}
		must(process.Process.Signal(syscall.SIGTERM))
		done := make(chan error, 1)
		go func() { done <- process.Wait() }()
		select {
		case e := <-done:
			must(e)
		case <-time.After(8 * time.Second):
			process.Process.Kill()
			t.Fatal("shutdown timeout")
		}
		process = nil
		processLog.Close()
	}
	defer func() {
		if process != nil {
			process.Process.Kill()
			process.Wait()
			processLog.Close()
		}
	}()
	transport := &http.Transport{DialContext: (&net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP("127.0.0.2")}}).DialContext}
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer transport.CloseIdleConnections()
	get := func(path string) (int, string, http.Header) {
		t.Helper()
		response, e := client.Get("http://127.0.0.1:8080" + path)
		if e != nil {
			return 0, "", nil
		}
		defer response.Body.Close()
		data, e := io.ReadAll(response.Body)
		must(e)
		return response.StatusCode, string(data), response.Header
	}
	wait := func(label string, condition func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if condition() {
				return
			}
			time.Sleep(40 * time.Millisecond)
		}
		logs, _ := os.ReadFile(filepath.Join(root, "service.log"))
		nginxLogs, _ := os.ReadFile(filepath.Join(root, "nginx.log"))
		t.Fatalf("timeout %s\n%s\n%s", label, logs, nginxLogs)
	}
	readReport := func() stats.PublicReport {
		_, body, _ := get("/data/stats.json")
		var report stats.PublicReport
		json.Unmarshal([]byte(body), &report)
		return report
	}
	views := func() int64 { return readReport().Pages["post:welcome"].Views }
	send := func(event int) {
		t.Helper()
		body := fmt.Sprintf(`{"v":1,"event_id":"%032x","page_id":"post:welcome","visitor_id":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","referrer_host":"search.example"}`, event)
		request, e := http.NewRequest("POST", "http://127.0.0.1:8080/api/events", strings.NewReader(body))
		must(e)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://127.0.0.1:8080")
		request.Header.Set("X-Forwarded-For", "192.0.2.99")
		request.Header.Set("Accept-Language", "zh-CN, en;q=0.8")
		request.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
		response, e := client.Do(request)
		must(e)
		defer response.Body.Close()
		if response.StatusCode != 204 {
			data, _ := io.ReadAll(response.Body)
			t.Fatalf("event: %d %s", response.StatusCode, data)
		}
	}
	database := func() *sql.DB {
		db, e := sql.Open("sqlite", "file:"+filepath.Join(home, "data/statistics/stats.db")+"?mode=ro")
		must(e)
		return db
	}
	start()
	wait("first run", func() bool { code, _, _ := get("/"); return code == 200 && readReport().BuildID != "" })
	for _, path := range []string{"/config.yaml", "/content/site.yaml", "/data/statistics/stats.db", "/statistics/visitor.key", "/meta/manifest.json", "/data/private/report.html", "/assets/", "/.git/config", "/data/../stats.db"} {
		code, _, _ := get(path)
		if code != 404 {
			t.Fatalf("private %s: %d", path, code)
		}
	}
	for path, location := range map[string]string{"/posts/welcome?x=1": "/posts/welcome/?x=1", "/posts/welcome/index.html?q=1": "/posts/welcome/?q=1", "/Legacy.html": "/posts/welcome/"} {
		code, _, headers := get(path)
		if code != 301 || headers.Get("Location") != location {
			t.Fatal("redirect", path, code, headers)
		}
	}
	code, _, _ := get("/missing/")
	if code != 404 {
		t.Fatal("404 status")
	}
	send(1)
	send(1)
	wait("initial event", func() bool { return views() == 1 })
	db := database()
	var count int
	must(db.QueryRow("SELECT count(*) FROM event_details").Scan(&count))
	db.Close()
	if count != 0 {
		t.Fatal("default details enabled")
	}
	// Wait for a rebuilt report, which follows the collector's config swap.
	oldBuild := readReport().BuildID
	config = strings.Replace(config, "enabled: false", "enabled: true", 1)
	must(os.WriteFile(configFile, []byte(config), 0644))
	wait("details reload", func() bool { report := readReport(); return report.BuildID != "" && report.BuildID != oldBuild })
	send(2)
	send(2)
	wait("details event", func() bool { return views() == 2 })
	db = database()
	var ip, browser, language, source string
	must(db.QueryRow("SELECT ip,browser,language,source FROM event_details").Scan(&ip, &browser, &language, &source))
	db.Close()
	if ip != "127.0.0.2" || browser != "Chrome" || language != "zh-CN" || source != "search.example" {
		t.Fatal("detail classification/proxy", ip, browser, language, source)
	}
	_, public, _ := get("/data/stats.json")
	if strings.Contains(public, "127.0.0.2") || strings.Contains(public, "Chrome") || strings.Contains(public, "review-copy") {
		t.Fatal("public privacy")
	}
	report, e := os.ReadFile(filepath.Join(home, "data/private/report.html"))
	must(e)
	if !bytes.Contains(report, []byte("127.0.0.2")) {
		t.Fatal("private report")
	}
	// Bad content keeps the last complete site; fixing it publishes new content.
	post := filepath.Join(home, "content/posts/welcome.md")
	original, e := os.ReadFile(post)
	must(e)
	oldBuild = readReport().BuildID
	must(os.WriteFile(post, []byte("invalid"), 0644))
	time.Sleep(600 * time.Millisecond)
	code, body, _ := get("/posts/welcome/")
	if code != 200 || readReport().BuildID != oldBuild {
		t.Fatal("bad content replaced site")
	}
	must(os.WriteFile(post, append(original, []byte("\nHot update accepted.\n")...), 0644))
	wait("hot rebuild", func() bool {
		_, body, _ := get("/posts/welcome/")
		return strings.Contains(body, "Hot update accepted.")
	})
	_ = body
	second := exec.Command(filepath.Join(home, "blog"), "serve", "--config", configFile)
	data, e := second.CombinedOutput()
	if e == nil || !bytes.Contains(data, []byte("locked")) {
		t.Fatal("second instance", e, string(data))
	}
	stop()
	keyPath := filepath.Join(home, "data/statistics/visitor.key")
	key, e := os.ReadFile(keyPath)
	must(e)
	backupDB, e := os.ReadFile(filepath.Join(home, "data/statistics/stats.db"))
	must(e)
	// Upgrade from an actual second extraction, copying only long-lived inputs/data.
	upgraded := unpack(filepath.Join(root, "upgrade"))
	for _, dir := range []string{"content", "data"} {
		must(os.RemoveAll(filepath.Join(upgraded, dir)))
		if data, e := exec.Command("cp", "-a", filepath.Join(home, dir), filepath.Join(upgraded, dir)).CombinedOutput(); e != nil {
			t.Fatalf("migrate %s: %v %s", dir, e, data)
		}
	}
	raw, e := os.ReadFile(configFile)
	must(e)
	must(os.WriteFile(filepath.Join(upgraded, "config.yaml"), raw, 0644))
	home = upgraded
	configFile = filepath.Join(home, "config.yaml")
	start()
	wait("upgrade restart", func() bool { return views() == 2 })
	next, e := os.ReadFile(filepath.Join(home, "data/statistics/visitor.key"))
	must(e)
	if !bytes.Equal(key, next) {
		t.Fatal("upgrade changed key")
	}
	send(2)
	time.Sleep(400 * time.Millisecond)
	if views() != 2 {
		t.Fatal("restart lost dedup")
	}
	send(3)
	wait("continued counts", func() bool { return views() == 3 })
	// A shortened detail retention cleans existing rows without changing aggregates.
	oldBuild = readReport().BuildID
	config = strings.Replace(config, "168h", "1ns", 1)
	must(os.WriteFile(configFile, []byte(config), 0644))
	wait("retention reload", func() bool { report := readReport(); return report.BuildID != "" && report.BuildID != oldBuild })
	db = database()
	must(db.QueryRow("SELECT count(*) FROM event_details").Scan(&count))
	db.Close()
	if count != 0 || views() != 3 {
		t.Fatal("retention changed aggregates", count, views())
	}
	stop()
	// Restore a stopped consistent DB and its original key, then start the package.
	must(os.WriteFile(filepath.Join(home, "data/statistics/stats.db"), backupDB, 0600))
	must(os.WriteFile(filepath.Join(home, "data/statistics/visitor.key"), key, 0600))
	start()
	wait("backup restore", func() bool { return views() == 2 })
	stop()
	fmt.Println(`BLOGO_RESULT {"passed":true,"uid":65534,"actual_archive":true,"proxy_ip":true,"upgrade":true,"backup_restore":true,"private_paths":true}`)
	t.Log("actual archive: nonroot serve, Nginx proxy/IP, default-off and hot details, retention, aliases/404, private files, failed/successful rebuild, restart dedup, upgrade and backup restore")
}
