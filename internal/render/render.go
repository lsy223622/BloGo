package render

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/markdown"
	"github.com/lsy223622/BloGo/internal/site"
	"github.com/tdewolff/minify/v2"
	minihtml "github.com/tdewolff/minify/v2/html"
)

type Renderer struct {
	config    *config.Config
	templates *template.Template
	prepared  map[string]markdown.Result
	pages     []*site.Page
	tax       *content.Taxonomies
	assets    map[string]string
}
type view struct {
	Site               config.Site
	Extra              map[string]any
	Page               *site.Page
	Body               template.HTML
	Summary, Canonical string
	TOC                []markdown.Heading
	Categories, Tags   []site.Term
	Recent             []*content.ContentItem
	Assets             map[string]string
	AboutURL, LinksURL string
	Mermaid            bool
}

func New(c *config.Config, pages []*site.Page, tax *content.Taxonomies, assets map[string]string) (*Renderer, error) {
	r := &Renderer{config: c, pages: pages, tax: tax, assets: assets, prepared: map[string]markdown.Result{}}
	for _, p := range pages {
		if p.Content == nil {
			continue
		}
		item := p.Content
		result := markdown.Result{HTML: item.Body, Summary: item.Meta.Summary}
		if item.Format == content.Markdown {
			var err error
			result, err = markdown.Convert(item.Body, item.Meta.Summary, c.Build.SummaryMaxChars)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", item.SourcePath, err)
			}
		}
		r.prepared[item.ID] = result
	}
	funcs := template.FuncMap{
		"date": func(t *time.Time) string {
			if t == nil {
				return ""
			}
			return t.Format("2006-01-02")
		},
		"summary":  func(item *content.ContentItem) string { return r.prepared[item.ID].Summary },
		"cover":    func(item *content.ContentItem) string { cover, _ := item.Meta.Extra["cover"].(string); return cover },
		"category": func(id string) string { return tax.Categories[id].Name },
		"tag":      func(id string) string { return tax.Tags[id].Name },
		"navCurrent": func(p *site.Page, target string) bool {
			if target == "/" {
				return p.ID == "home" || strings.HasPrefix(p.ID, "home:page:") || p.Kind == "post"
			}
			return strings.HasPrefix(p.URL, target)
		},
	}
	t := template.New("blog").Funcs(funcs)
	// Compact the template shell before inserting trusted author bodies. Register
	// only HTML so inline script/style text is kept byte-for-byte.
	compact := minify.New()
	compact.Add("text/html", &minihtml.Minifier{KeepComments: true, KeepDocumentTags: true, KeepEndTags: true, KeepQuotes: true, TemplateDelims: minihtml.GoTemplateDelims})
	err := fs.WalkDir(os.DirFS(c.Paths.Templates), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("template symlink: %s", p)
		}
		if filepath.Ext(p) != ".html" && filepath.Ext(p) != ".tmpl" {
			return fmt.Errorf("unsupported template: %s", p)
		}
		data, err := os.ReadFile(filepath.Join(c.Paths.Templates, filepath.FromSlash(p)))
		if err != nil {
			return err
		}
		data, err = compact.Bytes("text/html", data)
		if err != nil {
			return err
		}
		_, err = t.New(p).Parse(string(data))
		return err
	})
	if err != nil {
		return nil, err
	}
	if t.Lookup("page") == nil {
		return nil, fmt.Errorf("templates must define page")
	}
	r.templates = t
	return r, nil
}

func (r *Renderer) Render(p *site.Page) ([]byte, error) {
	if p.Content != nil && p.Content.Layout == "standalone" {
		script := ""
		if p.StatsEnabled && r.assets["stats.js"] != "" {
			var b bytes.Buffer
			t := template.Must(template.New("tracking").Parse(`<script src="{{.Src}}" data-page-id="{{.ID}}" data-site-timezone="{{.Timezone}}" defer></script>`))
			if err := t.Execute(&b, struct{ Src, ID, Timezone string }{r.assets["stats.js"], p.ID, r.config.Site.Timezone}); err != nil {
				return nil, err
			}
			script = b.String()
		}
		return PrepareStandalone(p.Content.Body, p.NoIndex, script)
	}
	v := view{Site: r.config.Site, Extra: r.config.Extra, Page: p, Canonical: strings.TrimRight(r.config.Site.BaseURL, "/") + p.URL, Assets: r.assets}
	if p.Content != nil {
		body := r.prepared[p.ID]
		v.Body = template.HTML(body.HTML)
		v.Summary = body.Summary
		v.TOC = body.TOC
		v.Mermaid = body.Mermaid
	}
	for _, page := range r.pages {
		if page.ID == "page:about" && page.Visibility == content.Public {
			v.AboutURL = page.URL
		}
		if page.ID == "page:links" && page.Visibility == content.Public {
			v.LinksURL = page.URL
		}
		if page.ID == "categories" {
			v.Categories = page.Terms
		}
		if page.ID == "tags" {
			v.Tags = page.Terms
		}
		if page.ID == "updates" {
			v.Recent = page.Items[:min(5, len(page.Items))]
		}
	}
	var b bytes.Buffer
	if err := r.templates.ExecuteTemplate(&b, "page", v); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
