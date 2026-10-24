package content

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path"
	"strings"
	"time"

	"github.com/lsy223622/BloGo/internal/yamlutil"
	"go.yaml.in/yaml/v3"
)

type Kind string

const (
	Post Kind = "post"
	Page Kind = "page"
)

type Format string

const (
	Markdown Format = "markdown"
	HTML     Format = "html"
)

// DateText preserves the author-supplied value until the site timezone is known.
type DateText string

func (d *DateText) UnmarshalYAML(n *yaml.Node) error {
	if n.Tag != "!!str" && n.Tag != "!!timestamp" {
		return yamlutil.Error(n, "date", "expected date or timestamp")
	}
	*d = DateText(n.Value)
	return nil
}

type Metadata struct {
	ID         string         `yaml:"id"`
	Title      string         `yaml:"title"`
	Slug       string         `yaml:"slug"`
	Permalink  string         `yaml:"permalink"`
	Aliases    []string       `yaml:"aliases"`
	Published  *DateText      `yaml:"published"`
	Updated    *DateText      `yaml:"updated"`
	Status     string         `yaml:"status"`
	Layout     string         `yaml:"layout"`
	Categories []string       `yaml:"categories"`
	Tags       []string       `yaml:"tags"`
	Summary    string         `yaml:"summary"`
	Stats      *bool          `yaml:"stats"`
	Extra      map[string]any `yaml:"extra"`
}

type ContentItem struct {
	SourcePath string
	Kind       Kind
	Format     Format
	Meta       Metadata
	Body       string
	ID         string
	Slug       string
	URL        string
	OutputPath string
	Aliases    []string
	Status     Status
	Layout     string
	Published  *time.Time
	Updated    *time.Time
	Stats      bool
	node       *yaml.Node
}

func (item *ContentItem) fieldError(field, message string) error {
	return fmt.Errorf("%s: %w", item.SourcePath, yamlutil.Error(yamlutil.Field(item.node, field), field, message))
}

func Parse(sourcePath string, data []byte) (*ContentItem, error) {
	fail := func(msg string) (*ContentItem, error) { return nil, fmt.Errorf("%s: %s", sourcePath, msg) }
	if !fs.ValidPath(sourcePath) || strings.Contains(sourcePath, "\\") {
		return fail("invalid source path")
	}
	parts := strings.Split(sourcePath, "/")
	if len(parts) < 2 || (parts[0] != "posts" && parts[0] != "pages") {
		return fail("expected posts/ or pages/ source path")
	}
	item := &ContentItem{SourcePath: sourcePath, Kind: Page, Format: Markdown}
	if parts[0] == "posts" {
		item.Kind = Post
	}
	switch path.Ext(sourcePath) {
	case ".md":
	case ".html":
		if item.Kind == Post {
			return fail("unsupported post file type")
		}
		item.Format = HTML
	default:
		return fail("unsupported source file type")
	}
	if !bytes.HasPrefix(data, []byte("---\n")) && !bytes.HasPrefix(data, []byte("---\r\n")) {
		return fail("expected opening Front Matter delimiter")
	}
	start := bytes.IndexByte(data, '\n') + 1
	for pos := start; pos <= len(data); {
		end := bytes.IndexByte(data[pos:], '\n')
		next := len(data)
		if end < 0 {
			end = len(data)
		} else {
			end += pos
			next = end + 1
		}
		if strings.TrimSuffix(string(data[pos:end]), "\r") == "---" {
			// Keep line numbers relative to the source, including the opening delimiter.
			frontMatter := data[start:pos]
			if len(bytes.TrimSpace(frontMatter)) == 0 {
				frontMatter = []byte("{}\n")
			}
			n, err := yamlutil.Decode(append([]byte("\n"), frontMatter...), &item.Meta)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", sourcePath, err)
			}
			item.node = n
			item.Body = string(data[next:])
			return item, nil
		}
		if next <= pos || next == len(data) {
			break
		}
		pos = next
	}
	return fail("missing closing Front Matter delimiter")
}

// ReadSources returns parsed sources in lexical relative-path order. os.Root
// confines reads even if a path changes while walking the tree.
func ReadSources(source string) ([]*ContentItem, error) {
	info, err := os.Lstat(source)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("%s: expected ordinary source directory", source)
	}
	root, err := os.OpenRoot(source)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	var items []*ContentItem
	err = fs.WalkDir(root.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: symbolic link is not allowed", p)
		}
		if d.IsDir() {
			if p == "assets" {
				return fs.SkipDir
			}
			if !strings.Contains(p, "/") && p != "posts" && p != "pages" {
				return fmt.Errorf("%s: unsupported source directory", p)
			}
			return nil
		}
		if p == "site.yaml" {
			return nil
		}
		i, err := d.Info()
		if err != nil {
			return err
		}
		if !i.Mode().IsRegular() {
			return fmt.Errorf("%s: expected ordinary file", p)
		}
		data, err := root.ReadFile(p)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		item, err := Parse(p, data)
		if err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return items, nil
}
