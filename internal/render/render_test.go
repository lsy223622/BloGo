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
		if p.ID != "categories" && p.ID != "page:about" {
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
		want := `href="/categories/" aria-current="location"`
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
