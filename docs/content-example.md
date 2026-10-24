---
id: article-format-example
title: BloGo 文章格式与 Markdown 语法示例
slug: article-format-example
status: draft
published: "2026-10-24T09:00:00+08:00"
updated: "2026-10-24T10:00:00+08:00"
layout: article
permalink: /posts/article-format-example/
aliases:
  - /old-article-format/
  - /legacy-format.html
categories:
  - tech
tags:
  - esp32
summary: >-
  完整的文章源文件示例，包含全部 Front Matter 字段、默认主题扩展数据，
  以及 BloGo 支持的 Markdown、目录、代码、公式和 Mermaid 语法。
stats: false
extra:
  cover: /images/example.svg
  links:
    - name: Go 项目
      url: https://go.dev/
      image: /images/avatar-default.svg
      description: 友链卡片示例，可按需删除整个 links 列表。
---

[TOC]

# 如何使用这份文件

这份文件本身就是一份可解析的文章源文件。复制到站点内容目录下的
`posts/article-format-example.md`，即可在本地预览中查看。写新文章时修改身份、
标题、日期和正文，并按需删减可选字段。当前状态为 `draft`。

顶部两个 `---` 之间是 YAML Front Matter，后面是 Markdown 正文。
使用 UTF-8 编码、空格缩进，列表每项单独一行。多行说明使用 `>-` 会把普通换行
折叠为空格；需要保留换行的字符串使用 `|`。字符串包含 `: `、`#` 等有特殊含义的
字符时，可以用引号包裹。

本文件展示全部字段，日常写作通常可以从更短的格式开始：

```yaml
---
id: my-first-post
title: 我的第一篇文章
status: draft
published: "2026-10-24T09:00:00+08:00"
categories:
  - tech
tags:
  - esp32
---

这里开始写正文。
```

本文件位于 `docs/`，不会被当作文章生成。实际内容目录由 `config.yaml` 的
`paths.content` 指定：项目示例为 `content/`，当前个人导入数据为
`output/personal-preview/content/`。图片根路径对应各自的 `paths.static`。

## 全部文章元数据

下面是程序支持的全部顶层 Front Matter 字段。没有需要的可选字段直接省略，
不必保留空字符串或空列表。顶层未知字段会报错，扩展展示数据放在 `extra`。

| 字段 | 格式、默认值与用途 |
| --- | --- |
| `id` | 稳定身份，例如 `my-first-post`；省略时使用解析后的 `slug`。只允许小写 ASCII 字母、数字和单个连字符分段，建议明确填写并保持稳定。内部身份按目录加上 `post:` 或 `page:`。 |
| `title` | 页面标题。`public`、`unlisted` 内容必须填写，草稿也建议填写。 |
| `slug` | URL 中的名称，规则与 `id` 相同。省略时从文件名去掉扩展名生成：大写转小写，空格及下划线转连字符。中文文件名需要显式设置 ASCII `slug`。 |
| `status` | `draft`、`public`、`unlisted`，默认 `draft`；行为见后面的状态表。 |
| `published` | 发布日期。公开和不列出的文章必须填写，独立页面和草稿可以省略。支持 `2026-10-24` 或 ISO 8601 / RFC 3339 时间，例如 `2026-10-24T09:00:00+08:00`。不带时区时按站点时区解释，不读取文件修改时间。 |
| `updated` | 更新日期，格式同 `published`，不能早于发布日期。省略且有发布日期时使用发布日期。影响更新列表与 Sitemap 的更新时间。 |
| `layout` | 文章 `.md` 默认 `article`；独立页面 `.md` 默认 `page`，也可选 `article`；独立页面 `.html` 默认 `standalone`，也可选 `page`。 |
| `permalink` | 自定义站内根路径，例如 `/notes/my-post/`。默认文章为 `/posts/{slug}/`，独立页面为 `/{slug}/`；嵌套页面保留 `pages/` 下的父目录。规范地址使用小写 ASCII 路径分段和末尾 `/`。 |
| `aliases` | 旧地址列表，生成重定向，例如 `/old-post/`、`/Legacy.html`。填写路径，不填完整域名、查询或片段；文件形式的别名不带末尾 `/`。 |
| `categories` | 分类 ID 列表，支持多个分类。对照 `content/site.yaml`的 `categories`；只有已配置的分类参与展示。都未配置时仍正常生成文章，作为无分类内容。 |
| `tags` | 标签 ID 列表，对照分类标签文件的 `tags`。只有已配置且被文章使用的标签参与生成与展示；公共标签列表按公开文章统计。 |
| `summary` | 手写摘要，非空时优先使用。省略或仅有空白时，Markdown 自动提取第一个有意义的段落，包括列表段落；保留行内代码文字，排除代码块、图片、公式、脚本和样式，并按 `content/site.yaml` 的 `summary_max_chars` 截断。HTML 源页面缺省为空摘要。 |
| `stats` | 布尔值 `true` / `false`，默认 `true`，控制该页面是否启用统计；统计服务需要另行配置。 |
| `extra` | 扩展映射。默认主题读取文章的 `cover`、`links`，格式见后文。自定义主题可读取其他扩展字段。 |

### 三种状态

| 状态 | 正式构建直接访问 | 公共发现入口 | 本地 `dev` 预览 |
| --- | --- | --- | --- |
| `public` | 可以 | 公开文章进入文章列表、RSS、分类标签与归档；公开文章和页面进入 Sitemap | 包含 |
| `unlisted` | 可以，普通页面输出 `noindex` | 不进入公共列表、RSS、Sitemap | 包含 |
| `draft` | 不生成页面 | 不进入 | 包含；预览首页可查看草稿文章 |

发布时将 `status` 改为 `public`，并填写 `published`。要保留访问地址但不列出，
使用 `unlisted`。这些状态由元数据决定，与文件名和文件时间无关。

### 分类与标签配置

文章中写 ID，显示名称在分类标签文件中设置，例如：

```yaml
schema_version: 1
categories:
  tech:
    name: 技术
  hardware:
    name: 硬件
    parent: tech
tags:
  esp32:
    name: ESP32
```

分类和标签 ID 使用小写 ASCII 字母、数字、连字符；显示名称可以包含中文、空格、
大小写和 emoji。分类还可设置 `description`、`parent`、`extra`，标签可设置
`description`、`extra`。实际使用的分类入口位于左侧导航。

### 封面与友链扩展

`extra.cover` 是文章列表旁的封面图地址，本地图片使用站点根路径：

```yaml
extra:
  cover: /images/example.svg
```

友链通常放在独立页面 `pages/links.md` 的 `extra.links` 中，按列表顺序生成卡片：

```yaml
extra:
  links:
    - name: 朋友的博客
      url: https://example.com/
      image: /images/avatar-default.svg
      description: 一句话介绍这个网站。
```

四个字段分别是名称、链接、头像、介绍。头像加载失败时使用本地默认头像。

### 独立页面与 HTML 页面

文章放在 `posts/`，独立页面放在 `pages/`，由目录决定类型。独立 Markdown 页面示例：

```yaml
---
id: about
title: 关于
slug: about
status: public
layout: page
permalink: /about/
stats: false
---

这里写页面正文，不要求填写发布日期。
```

`pages/` 还接受 `.html`：同样先写 Front Matter，再写 HTML 正文。
`layout: page` 将 HTML 片段放入主题；`layout: standalone` 保留完整 HTML 文档，
正文需要包含 `<!doctype html>`、`<html>`、`<head>`、`<body>`。
不列出的完整 HTML 页面需要在自己的 `<head>` 中声明：

```html
<meta name="robots" content="noindex">
```

站点名称、副标题、每页条数和侧栏站长信息属于站点配置，写在 `content/site.yaml`，
不写入文章 Front Matter。字段分别是 `title`、`subtitle`、
`page_size`、`profile`；可对照 `docs/configuration.md`、`docs/theme.md`。

# Markdown 语法示例

## 段落、换行与文字格式

空行分隔段落。需要强制换行时，在行末写两个空格或一个反斜杠：

第一行结尾是反斜杠。\
这里从新的一行开始。

支持 **加粗**、*斜体*、***加粗且斜体***、~~删除线~~ 和 `行内代码`。
例如版本号 `v4.3.0-beta`，在自动摘要里也会保留文字。

反斜杠可以转义标点：\*这不是斜体\*，\[这不是链接\]。
HTML 实体可以使用：`&amp;` 对应 &amp;，`&lt;` 对应 &lt;。

## 标题与目录

标题使用一至六个 `#`，后面留一个空格：

```markdown
# 一级标题
## 二级标题
### 三级标题
#### 四级标题
##### 五级标题
###### 六级标题
```

标题自动获得稳定、重复时去重的锚点。需要目录时，在正文中单独写一段：

```markdown
[TOC]

## 第一部分

正文……
```

标记前后用空行隔开，不区分大小写；标记本身不进入正文或自动摘要，目录按标题层级显示。
代码、引用和原生 HTML 中的标记不启用目录。

## 链接与图片

[Go 官方网站](https://go.dev/) 是普通链接。

支持引用式链接：[Go 网站][go-site]。

[go-site]: https://go.dev/ "Go 官方网站"

完整 URL 可以直接写成自动链接：<https://go.dev/>。

图片写法为 `![替代文字](图片地址)`，支持本地站点根路径和外部 HTTP(S) 地址：

![本地图片示例](/images/go.svg)

未包在链接中的正文图片可点击放大，关闭时缩回原位置。
附件使用普通链接语法；本地资源应放在 `paths.static` 下，从站点根路径引用。

## 列表和任务列表

- 无序列表第一项
- 无序列表第二项
  - 缩进后的子项
  - 同级子项

1. 有序列表第一项
2. 有序列表第二项

- [x] 已完成的任务
- [ ] 尚未完成的任务

## 引用与分隔线

> 这是一段引用。
>
> 可以分成多个段落。
>
> > 也可以嵌套引用。

下面是分隔线：

---

## 表格

竖线分隔单元格，分隔行中的冒号设置对齐方向：

| 左对齐 | 居中 | 右对齐 |
| :--- | :---: | ---: |
| 技术文章 | Go | 12 |
| 生活记录 | Markdown | 3 |

## 代码

反引号围栏后填写语言名称，构建时进行代码高亮：

```go
package main

import "fmt"

func main() {
    fmt.Println("Hello, BloGo!")
}
```

没有语言名称或未知语言时按纯文本处理：

```text
一段可以直接复制的文本。
代码块右侧有自动换行与复制按钮。
```

也支持每行四个空格的缩进代码块：

    echo "indented code"

需要在代码块中展示三反引号时，外层使用更多反引号：

````markdown
```go
fmt.Println("内层代码块")
```
````

## 数学公式

行内公式用单个美元符号包裹，例如 $E = mc^2$ 和 $a^2+b^2=c^2$。

块级公式使用独占一行的 `$$` 开始和结束：

$$
\sum_{i=1}^{n} i = \frac{n(n+1)}{2}
$$

公式由本地 KaTeX 在浏览器中增强；代码块和行内代码里的美元符号保持代码含义。

## Mermaid 图表

代码围栏的语言写成 `mermaid`，即可渲染图表：

```mermaid
flowchart LR
    A[编写 Markdown] --> B[Go 静态生成]
    B --> C[完整 HTML 页面]
    C --> D[浏览器阅读]
```

图表跟随页面深浅色；脚本不可用或图表语法有误时显示源码。

## 脚注

这里有一个脚注引用[^example]，脚注内容可以放在文末。

[^example]: 这是脚注的具体内容。

## 原生 HTML

Markdown 正文可以包含作者编写的原生 HTML，例如：

<details>
<summary>展开补充说明</summary>
<p>这里是保留原始结构的 HTML 内容。</p>
</details>

原生 HTML 被视为作者可信内容。完整文档使用前文说明的 `.html` 独立页面；
含作者脚本的页面沿用普通页面导航，确保脚本正常执行。
