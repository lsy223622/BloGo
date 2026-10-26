package render

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lsy223622/BloGo/internal/config"
	"github.com/lsy223622/BloGo/internal/content"
	"github.com/lsy223622/BloGo/internal/site"
)

func TestOrdinaryLayoutsEscapeMetadataAndPreserveTrustedBody(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tax, err := content.ReadTaxonomies(c.Paths.Taxonomies)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation(c.Site.Timezone)
	items, err := site.LoadContent(c.Paths.Source, c.Paths.Taxonomies, loc, false)
	if err != nil {
		t.Fatal(err)
	}
	index, err := site.NewIndex(items, false)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := site.BuildPages(index, tax, 10)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(c, pages, tax, map[string]string{"theme.css": "/assets/theme.css", "theme.js": "/assets/theme.js"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		if p.Content != nil && p.Content.Layout == "standalone" {
			continue
		}
		p.Title = `<script>alert("x")</script>`
		c.Site.Title = p.Title
		out, err := r.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(out)
		if !strings.Contains(s, "<!doctype html>") || !strings.Contains(s, "&lt;script&gt;") || strings.Contains(s, `<title><script>`) {
			t.Fatalf("escaping/document: %s", s)
		}
		if p.ID == "post:welcome" && !strings.Contains(s, "chroma") {
			t.Fatal("markdown body escaped")
		}
	}
}

func TestLogoAndSectionNavigation(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tax, err := content.ReadTaxonomies(c.Paths.Taxonomies)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation(c.Site.Timezone)
	items, err := site.LoadContent(c.Paths.Source, c.Paths.Taxonomies, loc, false)
	if err != nil {
		t.Fatal(err)
	}
	index, err := site.NewIndex(items, false)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := site.BuildPages(index, tax, 10)
	if err != nil {
		t.Fatal(err)
	}
	r, err := New(c, pages, tax, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		if p.ID != "category:tech" && p.ID != "page:about" {
			continue
		}
		data, err := r.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		s := string(data)
		if !strings.Contains(s, `class="brand-mark" aria-hidden="true"`) {
			t.Fatal("navigation logo missing")
		}
		if !strings.Contains(s, `href="/about/"`) || !strings.Contains(s, `href="/links/"`) {
			t.Fatal("page navigation missing")
		}
		nav := strings.SplitN(strings.SplitN(s, `<div class="nav-links">`, 2)[1], `</div>`, 2)[0]
		for _, categoryURL := range []string{"/categories/hardware/", "/categories/tech/"} {
			position := strings.Index(nav, `href="`+categoryURL+`"`)
			if position < strings.Index(nav, `href="/"`) || position > strings.Index(nav, `href="/tags/"`) {
				t.Fatal("category navigation must be between home and tags")
			}
		}
		want := `href="/categories/tech/" aria-current="location"`
		if p.ID == "page:about" {
			want = `href="/about/" aria-current="location"`
		}
		if !strings.Contains(s, want) {
			t.Fatalf("current section missing: %s", p.ID)
		}
	}
	for _, p := range pages {
		if p.ID == "page:about" || p.ID == "page:links" {
			p.Visibility = content.Unlisted
		}
	}
	home := &site.Page{ID: "home", URL: "/", Kind: "system", Title: "Home"}
	data, err := r.Render(home)
	if err != nil {
		t.Fatal(err)
	}
	nav := strings.SplitN(strings.SplitN(string(data), `<nav class="left-nav"`, 2)[1], `</nav>`, 2)[0]
	if strings.Contains(nav, `href="/about/"`) || strings.Contains(nav, `href="/links/"`) {
		t.Fatal("unlisted pages exposed in shared navigation")
	}
}

func TestMermaidScriptOnlyLoadsOnDiagramPages(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		body    string
		diagram bool
	}{{"```mermaid\nflowchart LR\nA --> B\n```", true}, {"```go\nprintln(1)\n```", false}} {
		item := &content.ContentItem{ID: "post:diagram", Format: content.Markdown, Body: test.body}
		p := &site.Page{ID: item.ID, URL: "/posts/diagram/", Content: item}
		r, err := New(c, []*site.Page{p}, &content.Taxonomies{}, map[string]string{"mermaid.js": "/vendor/mermaid.js"})
		if err != nil {
			t.Fatal(err)
		}
		out, err := r.Render(p)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(out), `src="/vendor/mermaid.js"`) != test.diagram {
			t.Fatalf("conditional diagram script: %s", out)
		}
	}
}

func TestSidebarProfileUsesTypedIconsAndEscapesMetadata(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c.Extra = map[string]any{"profile": []any{
		map[string]any{"type": "qq", "label": "123456789"},
		map[string]any{"type": "email", "label": "hello@example.com", "url": "mailto:hello@example.com"},
		map[string]any{"type": "github", "label": "<b>Writer</b>", "url": "https://github.com/example"},
		map[string]any{"type": "website", "label": "Website", "url": "https://example.com"},
		map[string]any{"type": "cc", "label": "CC BY-NC-SA 4.0", "url": "https://creativecommons.org/licenses/by-nc-sa/4.0/"},
	}}
	p := &site.Page{ID: "home", URL: "/", Kind: "system", Title: "Home"}
	pages := []*site.Page{p, {ID: "tags", Terms: []site.Term{{ID: "used", Name: "Used", URL: "/tags/used/"}}}}
	r, err := New(c, pages, &content.Taxonomies{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	profileStart := strings.Index(s, `<section class="site-profile"`)
	if profileStart < strings.Index(s, `</main>`) || profileStart < strings.Index(s, `<h2>标签</h2>`) || profileStart > strings.Index(s, `<section class="popular"`) {
		t.Fatal("profile must be in the shared sidebar after tags")
	}
	for _, kind := range []string{"qq", "email", "github", "website", "cc"} {
		if !strings.Contains(s, `/images/profile.svg#`+kind+`"`) {
			t.Fatalf("missing preset icon for %s", kind)
		}
	}
	if !strings.Contains(s, `&lt;b&gt;Writer&lt;/b&gt;`) || strings.Contains(s, `<b>Writer</b>`) {
		t.Fatal("profile links or metadata escaping")
	}
	if strings.Contains(s, "hello@example.com") || strings.Contains(s, "mailto:") || !strings.Contains(s, `data-email="`) {
		t.Fatal("profile email must be obfuscated without a mail link")
	}
	c.Extra = nil
	out, err = r.Render(p)
	if err != nil || strings.Contains(string(out), `class="site-profile"`) {
		t.Fatal("unconfigured profile rendered")
	}
}

func TestPartialNavigationKeepsPageIdentityInsideMain(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	p := &site.Page{ID: "post:visit", URL: "/posts/visit/", Title: "Visit", StatsEnabled: true}
	r, err := New(c, []*site.Page{p}, &content.Taxonomies{}, map[string]string{"stats.js": "/assets/stats.js", "navigation.js": "/assets/navigation.js", "swup.js": "/vendor/swup.js", "swup-scroll.js": "/vendor/scroll.js"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	main := strings.SplitN(strings.SplitN(s, `<main `, 2)[1], `</main>`, 2)[0]
	for _, want := range []string{`data-page-id="post:visit"`, `data-stats-src="/assets/stats.js"`, `data-swup-scroll-container`} {
		if !strings.Contains(main, want) {
			t.Fatalf("missing page context %s", want)
		}
	}
	for _, want := range []string{`src="/vendor/swup.js"`, `src="/vendor/scroll.js"`, `src="/assets/navigation.js"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing navigation runtime %s", want)
		}
	}
	if strings.Contains(main, `class="left-nav"`) || strings.Contains(main, `class="right-sidebar"`) || strings.Contains(main, `class="site-header"`) {
		t.Fatal("shared layout included in replacement container")
	}
}

func TestTagsWithoutPublicPagesRemainReadableWithoutBrokenLinks(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	tax := &content.Taxonomies{Tags: map[string]content.Tag{
		"shared":      {Name: "Shared"},
		"hidden-only": {Name: "Hidden only"},
	}}
	for _, visibility := range []content.Status{content.Unlisted, content.Draft} {
		t.Run(string(visibility), func(t *testing.T) {
			item := &content.ContentItem{ID: "post:entry", Format: content.Markdown, Meta: content.Metadata{Tags: []string{"shared", "hidden-only"}}}
			p := &site.Page{ID: item.ID, URL: "/posts/entry/", Content: item, Visibility: visibility}
			pages := []*site.Page{p, {ID: "tags", Terms: []site.Term{{ID: "shared", Name: "Shared", URL: "/tags/shared/"}}}}
			r, err := New(c, pages, tax, nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := r.Render(p)
			if err != nil {
				t.Fatal(err)
			}
			s := string(out)
			if strings.Contains(s, `href="/tags/hidden-only/"`) || !strings.Contains(s, `<span>Hidden only</span>`) {
				t.Fatal("tag without a public page must remain a readable label")
			}
			if !strings.Contains(s, `<a href="/tags/shared/">Shared</a>`) {
				t.Fatal("public tag link missing")
			}
		})
	}
}

func TestStandaloneTokenizerAndOriginalText(t *testing.T) {
	body := `<!doctype html><html><head><meta name="robots" content="noindex, follow"></head><body><!-- </body> --><script>const fake = "</body>";</script><p>Original  spacing</p></body></html>`
	script := `<script src="/assets/stats.js" defer></script>`
	out, err := PrepareStandalone(body, true, script)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != strings.Replace(body, "</p></body>", "</p>"+script+"</body>", 1) {
		t.Fatalf("unexpected edit %s", out)
	}
	out, err = PrepareStandalone(body, true, "")
	if err != nil || string(out) != body {
		t.Fatal("disabled tracking changed source")
	}
	for _, bad := range []string{`<html><head></head><body></body></html>`, `<html><head><!-- <meta name="robots" content="noindex"> --></head><body></body></html>`} {
		if _, err = PrepareStandalone(bad, true, ""); err == nil {
			t.Fatal("missing noindex accepted")
		}
	}
	if _, err = PrepareStandalone(`<html><head></head><body><script>"</body>"</script></html>`, false, script); err == nil {
		t.Fatal("fake body terminator accepted")
	}
}

func TestHTMLFragmentLayout(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	item := &content.ContentItem{ID: "page:fragment", URL: "/fragment/", Format: content.HTML, Layout: "page", Body: `<section data-test="raw"><script>window.fragment=true;</script></section>`}
	p := &site.Page{ID: item.ID, URL: item.URL, Content: item, Title: "Fragment", Visibility: content.Unlisted, NoIndex: true}
	r, err := New(c, []*site.Page{p}, &content.Taxonomies{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), item.Body) || !strings.Contains(string(out), `content="noindex"`) {
		t.Fatal("fragment or noindex missing")
	}
	// A malformed template fails before rendering a partial page.
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "page.html"), []byte("{{"), 0600); err != nil {
		t.Fatal(err)
	}
	c.Paths.Templates = dir
	if _, err = New(c, nil, &content.Taxonomies{}, nil); err == nil {
		t.Fatal("bad template accepted")
	}
}

func TestTemplateShellCompactionPreservesAuthorCodeAndBody(t *testing.T) {
	c, err := config.Load("../../config.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	c.Paths.Templates = dir
	script := "const exact = '  keep  ';\nwindow.value = exact;"
	style := ".custom { color: red; }\n"
	source := "{{define \"page\"}}<!doctype html>\n<html>\n    <head><script>" + script + "</script><style>" + style + "</style></head>\n    <body>{{.Body}}</body>\n</html>{{end}}"
	if err = os.WriteFile(filepath.Join(dir, "page.html"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	item := &content.ContentItem{ID: "page:raw", Format: content.HTML, Layout: "page", Body: "<div>  author spacing  </div><script>const raw = ' x '; </script>"}
	p := &site.Page{ID: item.ID, Content: item}
	r, err := New(c, []*site.Page{p}, &content.Taxonomies{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := r.Render(p)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if strings.Contains(s, "<html>\n") {
		t.Fatal("template shell not compacted")
	}
	if !strings.Contains(s, script) || !strings.Contains(s, style) || !strings.Contains(s, item.Body) {
		t.Fatalf("author source changed: %s", s)
	}
}
