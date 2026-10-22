package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParsePreservesBody(t *testing.T) {
	body := "<script>const tpl = '{{unsafe}}';</script>\r\n# 标题\r\n---\r\n"
	item, err := Parse("pages/guide/demo.html", []byte("---\r\ntitle: 示例\r\nextra:\r\n  nested: {value: 42}\r\n---\r\n"+body))
	if err != nil {
		t.Fatal(err)
	}
	if item.Body != body || item.Kind != Page || item.Format != HTML || item.Meta.Title != "示例" || item.Meta.Extra["nested"] == nil {
		t.Fatalf("parse: %+v", item)
	}
	if _, err = Parse("posts/nested/test.md", []byte("---\ntitle: Test\n---\nBody\n")); err != nil {
		t.Fatal(err)
	}
}

func TestEmptyFrontMatterDraft(t *testing.T) {
	item, err := Parse("posts/empty.md", []byte("---\n---\nBody"))
	if err != nil {
		t.Fatal(err)
	}
	if item.Body != "Body" {
		t.Fatalf("body %q", item.Body)
	}
}

func TestParseRejectsBadSources(t *testing.T) {
	for _, tc := range []struct{ path, data, want string }{
		{"posts/a.md", "title: No delimiter", "Front Matter"},
		{"posts/a.md", "---\ntitle: Test\n", "closing"},
		{"posts/a.md", "---\ntitel: Typo\n---\n", "titel"},
		{"posts/a.md", "---\nstats: 'true'\n---\n", "stats"},
		{"posts/a.md", "---\ntitle: A\ntitle: B\n---\n", "title"},
		{"posts/a.md", "---\ntags: [1]\n---\n", "tags"},
		{"posts/a.html", "---\ntitle: A\n---\n", "type"},
		{"pages/a.txt", "---\ntitle: A\n---\n", "type"},
		{"../pages/a.md", "---\ntitle: A\n---\n", "path"},
	} {
		t.Run(tc.want+tc.path, func(t *testing.T) {
			_, err := Parse(tc.path, []byte(tc.data))
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), tc.path) {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func writeSource(t *testing.T, root, name, data string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestReadSources(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "posts/sub/b.md", "---\ntitle: B\n---\nB")
	writeSource(t, root, "pages/a.html", "---\ntitle: A\n---\n<html></html>")
	items, err := ReadSources(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].SourcePath != "pages/a.html" {
		t.Fatalf("items: %+v", items)
	}
	writeSource(t, root, "posts/bad.txt", "bad")
	if _, err := ReadSources(root); err == nil {
		t.Fatal("accepted unsupported file")
	}
}

func TestReadSourcesRejectsSymlinks(t *testing.T) {
	root := t.TempDir()
	writeSource(t, root, "posts/a.md", "---\ntitle: A\n---\n")
	if err := os.Symlink(filepath.Join(root, "posts/a.md"), filepath.Join(root, "posts/link.md")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := ReadSources(root); err == nil {
		t.Fatal("accepted symlink")
	}
}
