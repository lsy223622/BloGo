package markdown

import (
	"reflect"
	"strings"
	"testing"
)

func TestMarkdownFeaturesAndStableTOC(t *testing.T) {
	source := "# 标题\n\n## Repeat\n\n## Repeat\n\n正文 **强调** [链接](/about/) ![图](/images/a.png).\n\n> 引用\n\n- [x] 完成\n- [ ] 待办\n\n| a | b |\n| - | - |\n| 1 | 2 |\n\n~~删除~~[^1]\n\n[^1]: 脚注\n\n```go\nfmt.Println(42)\n```\n\n<div class=\"custom\">HTML</div>\n\n公式 $x < y$。\n\n$$\nE = mc^2\n$$\n"
	a, err := Convert(source, "", 160)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Convert(source, "", 160)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) || len(a.TOC) != 3 || a.TOC[1].ID == a.TOC[2].ID {
		t.Fatalf("unstable TOC: %+v", a.TOC)
	}
	for _, fragment := range []string{"<table>", "checkbox", "<del>", "footnote", "<blockquote>", "<img ", "<div class=\"custom\">HTML</div>", "chroma", "math-inline", "math-block"} {
		if !strings.Contains(a.HTML, fragment) {
			t.Errorf("missing %s in %s", fragment, a.HTML)
		}
	}
}

func TestSummaryUsesFirstMeaningfulASTParagraph(t *testing.T) {
	source := "# Heading\n\n```go\nsecretCode()\n```\n\n<script>secretScript()</script>\n\n<style>.secret{}</style>\n\n$$\nsecretMath\n$$\n\n中文 &amp; **正文** `secretInline` $secretFormula$ <script>inlineSecret()</script> 后文。\n\nSecond paragraph."
	r, err := Convert(source, "", 7)
	if err != nil {
		t.Fatal(err)
	}
	if r.Summary != "中文 & 正文" {
		t.Fatalf("summary %q", r.Summary)
	}
	r, err = Convert(source, " 手写  摘要 ", 160)
	if err != nil || r.Summary != "手写 摘要" {
		t.Fatalf("explicit summary: %+v %v", r, err)
	}
}

func TestSummaryFromTightListSkipsImageOnlyParagraph(t *testing.T) {
	for _, body := range []string{
		"# 标题\n\n- 迁移网站的注意事项。\n- 下一条。\n",
		"![2025-07-01T15:45:29.png](https://example.com/image.png)\n\n迁移网站的注意事项。\n",
	} {
		r, err := Convert(body, "", 160)
		if err != nil || r.Summary != "迁移网站的注意事项。" {
			t.Errorf("summary %q: %v", r.Summary, err)
		}
	}
}
