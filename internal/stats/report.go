package stats

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/filelock"
	"github.com/lsy223622/BloGo/internal/site"
)

type PageViews struct {
	Views int64 `json:"views"`
}
type PopularPage struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Views int64  `json:"views"`
}
type PublicReport struct {
	SchemaVersion int                  `json:"schema_version"`
	BuildID       string               `json:"build_id"`
	GeneratedAt   time.Time            `json:"generated_at"`
	Pages         map[string]PageViews `json:"pages"`
	Popular30D    []PopularPage        `json:"popular_30d"`
}
type counts struct{ All, Seven, Thirty int64 }
type trend struct {
	Day    string
	PV, UV int64
}
type sourceCount struct {
	Source string
	counts
}
type privatePage struct {
	ID, Title, URL, Visibility string
	counts
	Sources []sourceCount
}
type reportSnapshot struct {
	Details     []DetailEvent
	DetailCount int
	PV, UV      counts
	Days        []trend
	Pages       map[string]counts
	Sources     []sourceCount
	PageSources map[string][]sourceCount
}

func (s *Store) snapshot(ctx context.Context, today, seven, thirty string) (reportSnapshot, error) {
	result := reportSnapshot{Pages: map[string]counts{}, PageSources: map[string][]sourceCount{}}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, "SELECT day,pv,uv FROM daily_site_stats ORDER BY day")
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var d trend
		if err = rows.Scan(&d.Day, &d.PV, &d.UV); err != nil {
			rows.Close()
			return result, err
		}
		result.Days = append(result.Days, d)
		result.PV.All += d.PV
		result.UV.All += d.UV
		if d.Day <= today && d.Day >= seven {
			result.PV.Seven += d.PV
			result.UV.Seven += d.UV
		}
		if d.Day <= today && d.Day >= thirty {
			result.PV.Thirty += d.PV
			result.UV.Thirty += d.UV
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT page_id,sum(pv),sum(CASE WHEN day BETWEEN ? AND ? THEN pv ELSE 0 END),sum(CASE WHEN day BETWEEN ? AND ? THEN pv ELSE 0 END) FROM daily_page_stats GROUP BY page_id`, seven, today, thirty, today)
	if err != nil {
		return result, err
	}
	for rows.Next() {
		var id string
		var c counts
		if err = rows.Scan(&id, &c.All, &c.Seven, &c.Thirty); err != nil {
			rows.Close()
			return result, err
		}
		result.Pages[id] = c
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	rows, err = tx.QueryContext(ctx, `SELECT page_id,source,sum(pv),sum(CASE WHEN day BETWEEN ? AND ? THEN pv ELSE 0 END),sum(CASE WHEN day BETWEEN ? AND ? THEN pv ELSE 0 END) FROM daily_referrers GROUP BY page_id,source ORDER BY page_id,sum(pv) DESC,source`, seven, today, thirty, today)
	if err != nil {
		return result, err
	}
	totals := map[string]counts{}
	for rows.Next() {
		var id string
		var source sourceCount
		if err = rows.Scan(&id, &source.Source, &source.All, &source.Seven, &source.Thirty); err != nil {
			rows.Close()
			return result, err
		}
		result.PageSources[id] = append(result.PageSources[id], source)
		c := totals[source.Source]
		c.All += source.All
		c.Seven += source.Seven
		c.Thirty += source.Thirty
		totals[source.Source] = c
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	if err = rows.Close(); err != nil {
		return result, err
	}
	for source, c := range totals {
		result.Sources = append(result.Sources, sourceCount{source, c})
	}
	sort.Slice(result.Sources, func(i, j int) bool {
		a, b := result.Sources[i], result.Sources[j]
		if a.All != b.All {
			return a.All > b.All
		}
		return a.Source < b.Source
	})
	result.Details, result.DetailCount, err = readDetails(ctx, tx)
	if err != nil {
		return result, err
	}
	return result, tx.Commit()
}

func GenerateReport(ctx context.Context, c *config.Config) error {
	return generateReport(ctx, c, time.Now(), nil)
}

func generateReport(ctx context.Context, c *config.Config, now time.Time, beforePublish func()) error {
	if err := c.Validate("report", ""); err != nil {
		return err
	}
	targets := []string{c.Stats.PublicOutput, c.Stats.PrivateOutput}
	sort.Strings(targets)
	var locks []*filelock.Lock
	defer func() {
		for _, lock := range locks {
			lock.Close()
		}
	}()
	for _, target := range targets {
		mode := os.FileMode(0700)
		if target == c.Stats.PublicOutput {
			mode = 0755
		}
		if err := os.MkdirAll(filepath.Dir(target), mode); err != nil {
			return err
		}
		lock, err := filelock.Acquire(target + ".lock")
		if err != nil {
			return err
		}
		locks = append(locks, lock)
	}
	manifest, err := site.ReadManifest(c.Stats.ManifestPath)
	if err != nil {
		return err
	}
	zone, err := time.LoadLocation(c.Site.Timezone)
	if err != nil {
		return err
	}
	now = now.In(zone)
	readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	store, err := OpenReader(readCtx, c.Stats.DatabasePath)
	if err != nil {
		return err
	}
	snapshot, err := store.snapshot(readCtx, now.Format("2006-01-02"), now.AddDate(0, 0, -6).Format("2006-01-02"), now.AddDate(0, 0, -29).Format("2006-01-02"))
	closeErr := store.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	public := PublicReport{1, manifest.BuildID, now, map[string]PageViews{}, []PopularPage{}}
	records := map[string]site.PageRecord{}
	for _, p := range manifest.Pages {
		records[p.ID] = p
		if p.Visibility != content.Public || !p.StatsEnabled {
			continue
		}
		views := snapshot.Pages[p.ID]
		public.Pages[p.ID] = PageViews{views.All}
		if p.Kind == "post" {
			public.Popular30D = append(public.Popular30D, PopularPage{p.ID, p.Title, p.URL, views.Thirty})
		}
	}
	sort.Slice(public.Popular30D, func(i, j int) bool {
		a, b := public.Popular30D[i], public.Popular30D[j]
		if a.Views != b.Views {
			return a.Views > b.Views
		}
		return a.ID < b.ID
	})
	privateIDs := map[string]bool{}
	for id := range snapshot.Pages {
		privateIDs[id] = true
	}
	for _, p := range manifest.Pages {
		if p.StatsEnabled {
			privateIDs[p.ID] = true
		}
	}
	var pages []privatePage
	for id := range privateIDs {
		p, ok := records[id]
		item := privatePage{ID: id, Title: id + "（已删除）", Visibility: "已删除", counts: snapshot.Pages[id], Sources: snapshot.PageSources[id]}
		if ok {
			item.Title = p.Title
			item.URL = p.URL
			item.Visibility = string(p.Visibility)
			if !p.StatsEnabled {
				item.Visibility += " / 统计关闭"
			}
		}
		pages = append(pages, item)
	}
	sort.Slice(pages, func(i, j int) bool {
		if pages[i].All != pages[j].All {
			return pages[i].All > pages[j].All
		}
		return pages[i].ID < pages[j].ID
	})
	data, err := json.MarshalIndent(public, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	for i := range snapshot.Details {
		snapshot.Details[i].RecordedAt = snapshot.Details[i].RecordedAt.In(zone)
	}
	var html bytes.Buffer
	model := struct {
		Title, BaseURL, BuildID, Generated string
		PV, UV                             counts
		Days                               []trend
		Pages                              []privatePage
		Sources                            []sourceCount
		Details                            []DetailEvent
		DetailCount                        int
		DetailsEnabled                     bool
	}{c.Site.Title, c.Site.BaseURL, manifest.BuildID, now.Format(time.RFC3339), snapshot.PV, snapshot.UV, snapshot.Days, pages, snapshot.Sources, snapshot.Details, snapshot.DetailCount, c.Stats.Details.Enabled}
	if err = privateReportTemplate.Execute(&html, model); err != nil {
		return err
	}
	publicTemp, err := reportTemp(c.Stats.PublicOutput, data, 0644)
	if err != nil {
		return err
	}
	defer os.Remove(publicTemp)
	privateTemp, err := reportTemp(c.Stats.PrivateOutput, html.Bytes(), 0600)
	if err != nil {
		return err
	}
	defer os.Remove(privateTemp)
	if beforePublish != nil {
		beforePublish()
	}
	current, err := site.ReadManifest(c.Stats.ManifestPath)
	if err != nil {
		return err
	}
	if current.BuildID != manifest.BuildID {
		return fmt.Errorf("manifest changed during report generation")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Rename(publicTemp, c.Stats.PublicOutput); err != nil {
		return err
	}
	return os.Rename(privateTemp, c.Stats.PrivateOutput)
}

func reportTemp(target string, data []byte, mode os.FileMode) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(target), ".blogo-report-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	if err = file.Chmod(mode); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(name)
		return "", err
	}
	return name, nil
}

var privateReportTemplate = template.Must(template.New("report").Parse(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} · 统计报告</title><style>body{font:16px/1.7 system-ui,sans-serif;color:#23332e;background:#f5f7f5;margin:0}main{max-width:1100px;margin:auto;padding:32px 20px}table{border-collapse:collapse;width:100%;background:white;margin:20px 0}th,td{text-align:left;padding:8px 12px;border-bottom:1px solid #dce4de;overflow-wrap:anywhere}th{background:#e8efea}h1,h2,h3{line-height:1.3}small{color:#52665b}a{color:#266b50}.scroll{overflow:auto}code{overflow-wrap:anywhere}details{padding:8px;background:white;margin:12px 0}</style></head><body><main><h1>{{.Title}} · 私有统计报告</h1><p>版本 <code>{{.BuildID}}</code><br>生成时间 {{.Generated}}</p><p>UV 为各日独立访客数之和，不表示跨日去重人数。</p><table><tr><th>指标</th><th>7 天</th><th>30 天</th><th>累计</th></tr><tr><td>PV</td><td>{{.PV.Seven}}</td><td>{{.PV.Thirty}}</td><td>{{.PV.All}}</td></tr><tr><td>UV（日合计）</td><td>{{.UV.Seven}}</td><td>{{.UV.Thirty}}</td><td>{{.UV.All}}</td></tr></table><h2>每日趋势</h2><div class="scroll"><table><tr><th>日期</th><th>PV</th><th>UV</th></tr>{{range .Days}}<tr><td>{{.Day}}</td><td>{{.PV}}</td><td>{{.UV}}</td></tr>{{end}}</table></div><h2>来源总量</h2><table><tr><th>主机名</th><th>7 天</th><th>30 天</th><th>累计</th></tr>{{range .Sources}}<tr><td>{{.Source}}</td><td>{{.Seven}}</td><td>{{.Thirty}}</td><td>{{.All}}</td></tr>{{end}}</table><h2>页面与来源</h2>{{range .Pages}}<details><summary>{{.Title}} · {{.All}} PV</summary><p><code>{{.ID}}</code> · {{.Visibility}}{{if .URL}} · <a href="{{$.BaseURL}}{{.URL}}">{{.URL}}</a>{{end}}</p><p>7 天 {{.Seven}} · 30 天 {{.Thirty}} · 累计 {{.All}}</p><table><tr><th>来源</th><th>7 天</th><th>30 天</th><th>累计</th></tr>{{range .Sources}}<tr><td>{{.Source}}</td><td>{{.Seven}}</td><td>{{.Thirty}}</td><td>{{.All}}</td></tr>{{end}}</table></details>{{end}}{{if or .DetailsEnabled .DetailCount}}<h2>最近事件</h2><p>保留 {{.DetailCount}} 条明细，最多显示最近 1000 条。{{if not .DetailsEnabled}}详细记录已关闭，现有明细按保留期清理。{{end}}</p><div class="scroll"><table><tr><th>时间</th><th>页面</th><th>IP</th><th>来源</th><th>浏览器 / 系统 / 设备</th><th>语言</th><th>UA 与事件身份</th></tr>{{range .Details}}<tr><td>{{.RecordedAt.Format "2006-01-02 15:04:05Z07:00"}}</td><td>{{.Title}}<br><code>{{.PageID}}</code><br>{{.URL}}</td><td>{{.IP}}</td><td>{{.Source}}</td><td>{{.Browser}} / {{.OS}} / {{.Device}}</td><td>{{.Language}}</td><td><details><summary>详情</summary><p>{{.UserAgent}}</p><p>事件 {{.EventID}}<br>当日访客摘要 {{.VisitorDigest}}</p></details></td></tr>{{end}}</table></div>{{end}}</main></body></html>`))
