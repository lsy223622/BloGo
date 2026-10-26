# 搜索抓取、旧地址兼容与全文订阅优化

日期：2026-10-26。状态：B—D 已有本地实现；用户进一步要求将 A 修订为通用的别名通配符机制，A 及整体集成须按本版重新实施和验收。

## 目标与边界

提供可由站点作者配置的通用别名通配符机制，补齐常见订阅入口，提供全文订阅，完善自动生成的页面元数据与分页标题。保持极简部署，不增加额外服务、后台、常驻任务或依赖升级。

- 以当前仓库和 main 的实际状态为基线，先检查分支、未提交工作及适用 AGENTS.md；保留已有工作。
- v1.0.1 已发布，main 包含 Go 1.27.3 工具链安全修复。不得退回旧工具链或沿用过时开发分支覆盖新提交。
- 本轮授权代码、模板、必要文档及测试的修改与本地验证；不自动推送、合并、打标签、发布 Release 或部署线上，不修改服务器配置与个人博文目录。
- 用户接受全文订阅中的阅读无法完整计入站内阅读量。
- 用户明确要求以前文章的标题层级保持原样；不批量改正文、不恢复评论功能、不增加 llms.txt、不更改 AI 训练/搜索抓取政策。
- 小范围完整实现，复用现有构建、模板、路由与配置约定；只扩展既有 aliases，不顺带重构或引入全功能迁移框架。
- 通用程序不内置特定博客系统的文章地址、评论分页规则或个人站点数据。公共代码、测试和使用文档使用虚构的中性路径；部署实例的真实旧地址规则只放在用户自己的内容配置。

## 现状与方案取舍

部署实例的检查表明，基础 HTML 抓取、sitemap 和 canonical 正常。待改善的是旧地址的批量别名匹配、订阅格式、首页元数据和分页标题；实例检查数据不作为通用程序中的路由常量。

选定方案：

1. 在文章既有 aliases 中支持明确声明的通配模式，匹配后跳向所属文章 canonical。逐条枚举维护成本高，内置某博客系统的规则会污染通用产品；匹配范围由用户自己的规则决定，不自动扩展精确别名的后缀。
2. feed 按原格式直接返回 200。用户选择避免依赖阅读器跳转能力；不能把 RSS 1.0、RSS 2.0 和 Atom 全部当成相同 XML。
3. 默认全文，摘要同时保留。现有公开文章选择与排序继续使用，不增加全文/摘要模式开关或新的条目数量策略。
4. OG、HTML title、JSON-LD 由已有内容数据自动生成，不要求作者逐篇维护新的重复元数据。

官方依据：

- 历史订阅格式参考：https://github.com/typecho/typecho/blob/master/var/Widget/Feed.php
- RSS 2.0：https://www.rssboard.org/rss-specification
- RSS 摘要及全文：https://www.rssboard.org/rss-profile
- Atom 1.0：https://www.rfc-editor.org/rfc/rfc4287
- Open Graph：https://ogp.me/
- Google Article 数据：https://developers.google.com/search/docs/appearance/structured-data/article
- Google 地址迁移：https://developers.google.com/search/docs/crawling-indexing/site-move-with-url-changes

## 1. 通用别名通配符

扩展文章现有 Front Matter 的 aliases，使每个条目既可以是原来的精确路径，也可以是作者明确声明的通配路径。目标仍是这篇文章的规范 URL，不增加全局重写表、捕获组替换或模板执行。

以下为虚构示例：

```yaml
aliases:
  - /legacy/example/
  - /legacy/example/replies-*
```

第二条模式可以匹配 `/legacy/example/replies-1`、`/legacy/example/replies-27` 等，直接 301 到所属文章 canonical。它也能匹配非数字后缀，这是通用 `*` 的定义；程序不额外理解“评论页码”。站点作者自行选择需要覆盖的路径范围。

- 第一阶段只支持 `*`，匹配一个路径段内的零个或多个字符，不跨 `/`；必须匹配整个请求路径。暂不增加 `**`、正则表达式或捕获替换功能。
- 无通配符的 aliases 保持原有规范化和精确匹配行为。通配模式的尾斜杠行为须明确写入使用文档并测试，不得悄悄扩展成任意子路径匹配。
- 只有内容中显式声明了通配别名才生效；旧文章存在精确 alias 本身不会自动启用任何附加规则。
- 普通规范页面、已存在的静态文件、系统入口和精确别名优先于通配别名，不允许通配规则劫持它们。
- 多个通配模式的优先级必须明确、稳定并写入文档，不得依赖 Go map 遍历或扫描偶然顺序；相同模式指向不同文章时，构建报冲突。
- 复用现有私有路径、站内路径及冲突校验；不接受外部域名作为源、路径穿越、查询字符串注入或绕过私有文件限制。
- 一次 301 直接到所属文章 canonical，沿用现有查询字符串保留策略。未知且未匹配的路径维持原 404。
- 模式随构建产物/发布快照更新，Linux serve 与 Windows dev 行为一致；不得只在 HTTP 层用写死规则临时兜底。
- 本轮只实现并验证通用能力，不连接服务器或修改个人内容配置。部署实例的具体旧地址规则后续由用户配置。

## 2. 订阅入口及格式

| URL | 格式 | Content-Type | 响应 |
| --- | --- | --- | --- |
| `/rss.xml` | RSS 2.0 | `application/rss+xml; charset=utf-8` | 200，主订阅地址 |
| `/feed/` | RSS 2.0 | `application/rss+xml; charset=utf-8` | 200，与 `/rss.xml` 相同内容 |
| `/feed/rss/` | RSS 1.0 / RDF | `application/rdf+xml; charset=utf-8` | 200 |
| `/feed/atom/` | Atom 1.0 | `application/atom+xml; charset=utf-8` | 200 |

兼容入口直接服务构建生成的静态内容，不通过 301 转到其他格式；可以生成对应目录的 index.xml 等文件并以少量固定映射服务，沿用现有架构自行选取最小实现，不要求复制重复的 RSS 2.0 文件。

- 页面中现有订阅发现链接、页脚仍以 `/rss.xml` 为主；不必在界面展示三套选择。
- 无尾斜杠形式可以直接服务同一内容；如沿用规范路径跳转，不得改变以上四个列明入口直接 200 的要求。
- 不扩展到旧评论 feed、任意分类/标签 feed；不新增动态 feed API。
- 新增输出位置需要参与保留路由、静态文件冲突和构建输出校验，避免静默覆盖作者文件。
- GET/HEAD 应正确工作；HEAD 不返回正文，媒体类型准确。

## 3. 全文与条目身份

三种格式复用同一批公开文章、现有顺序、摘要与正文转换，不改文章 ID、slug、发布时间或现有 RSS 2.0 GUID。

- RSS 2.0：description 保留摘要，content:encoded 提供完整 HTML，声明正确命名空间，description 在前。
- RSS 1.0：输出有效 RDF feed、条目及顺序关系，使用合适扩展字段承载全文。
- Atom：提供稳定合法的条目 ID、标题、链接、发布时间/更新时间、summary 与 HTML content；作者信息遵循格式要求并使用真实配置/站点身份，不编造个人姓名。
- 全文只包含文章正文，不含导航、侧栏、统计和开发重载脚本；不要将整张渲染页面作为 feed 正文。
- Markdown 使用现有正文渲染结果或等价的同一转换规则，不输出原始 Markdown 充当 HTML。
- 正文中的相对链接及图片地址按文章 URL 和站点 base_url 正确解析为绝对 URL，保留外部地址和锚点语义。不能使用字符串拼接误处理 `../`。
- 正确编码 XML/内嵌 HTML，覆盖中文、emoji、`&`、尖括号和 `]]>`；输出须能被 XML 解析器及对应格式结构检查读取。
- 继续排除 draft、unlisted、未来尚未发布的文章和独立页面，不能因 dev 可预览草稿而把它们加入公开 feed。
- JSON-LD、Feed 中的更新时间来自实际内容信息，不能用每次构建时间伪装文章更新。缺省更新时间按格式必要性合理使用实际发布日期，不改变源数据。
- Mermaid 等依赖 JS 的内容不要求在阅读器中复现交互；保留可阅读的正文和代码，不引入图像预生成服务。
- RSS 阅读不补发或伪造站内统计事件。

## 4. 标题、简介与 Open Graph

复用 content/site.yaml 的 title、subtitle 和现有文章摘要/分类标签描述。无需新增一套必须填写的 SEO 配置。

- 首页 HTML title 和 og:title 使用站名，description 与 og:description 使用站点 subtitle；现有默认 subtitle 已非空，不硬编码个人站点介绍到仓库。
- 文章使用文章标题与现有摘要，OG 地址继续使用规范绝对地址。
- 分类/标签/普通列表使用能说明页面内容的标题；标签页应带“标签”等上下文，区分 `/links/` 与 `/tags/friends/` 之类同名页面。
- 分页 title 与 OG title 包含当前页码；正文列表标题的页码按下一节显示。
- 有真实分类/标签描述时优先使用，其他列表可复用站点简介，避免输出无意义空 OG 描述；不生成堆砌关键词文案。
- 元数据 HTML 转义必须正确，不通过拼接用户文本输出未转义标签。

## 5. 分页外观

当 `Pager.Total > 1` 时，在列表可见主标题右侧同一行显示小字号、弱化颜色的“第 N / M 页”；包括第 1 页。总页数为 1 时不显示。窄屏允许自然换行，不挤压溢出。

例：主标题“技术”，旁边小字“第 2 / 2 页”。底部上一页/下一页导航和页数照旧。

浏览器标签/搜索参考的 title 是纯文本，例如“技术 · 第 2 页 · 站点名”，不在 title 内放 HTML 或样式。各页 canonical 仍指向自身，不全部指向第 1 页。

Swup 导航后应同步 title、OG/description、canonical、分页标题和结构化数据；不能让上一页文章数据留在下一页。

## 6. JSON-LD

普通模板渲染的公开文章在 head 中生成 `application/ld+json` 的 BlogPosting，直接存在初始 HTML，浏览器无需执行脚本。

- 至少包括 @context、@type、headline、规范 url、datePublished；有实际更新信息时加 dateModified。
- 作者、描述、图片等仅从已有真实元数据/配置取；已有模型无作者字段时可省略，不为补齐可选属性扩展整套配置，更不能把站名假装成个人姓名。
- 图片仅在存在相关文章图片时使用绝对 URL；不随意拿 logo 当文章配图。
- 采用可靠 JSON 编码，正确处理引号、换行、中文与 `</script>`，避免脚本标签被文本提前闭合。
- 不把模板管理扩展到作者控制的 standalone HTML，不在分类/归档页错误生成 BlogPosting。
- Swup 站内切换须移除/替换旧文章 JSON-LD，文章切换到列表时不残留 BlogPosting。

## 实施与验收清单

执行方式：用户已明确指定现有开发会话执行，无需重新请求方案批准。使用 writing-plans / executing-plans 按下列任务顺序完成，实施细节沿用项目约定。

### A. 通用别名通配符

参考文件：`internal/content/identity.go`、`internal/site/routes.go`、`internal/site/seo.go`、构建产物元数据及 `internal/server/http.go`，以及对应测试。

- [ ] 移除本轮专用路由、对应专用测试及公共文档中的真实站点/文章地址，不回滚其他功能。
- [ ] 从 aliases 解析、构建校验与持久化到共享 Handler，实现通用 `*` 模式；声明精确路由、通配模式的匹配优先级及尾斜杠行为。
- [ ] 回归覆盖：虚构的不同旧路径格式、显式声明与未声明的区别、单路径段边界、零字符匹配、尾斜杠、查询字符串、直接最终 Location；精确页面/静态文件/系统入口/精确别名优先；重复模式冲突、重叠规则确定性、私有路径隔离与热更新。
- [ ] 更新别名使用文档，例子全部使用中性虚构数据；设计中的具体博客迁移规则不得变成产品默认值。

### B. 三种全文 feed 与静态入口

参考文件：`internal/site/seo.go`（必要时拆出小型 feed 文件）、`internal/build/build.go`、`internal/site/routes.go`、`internal/server/http.go` 及对应测试。

- [x] 用同一公开文章集合生成 RSS 2.0、RSS 1.0、Atom，接入构建结果与 HTTP 入口。
- [x] 覆盖 XML 格式结构、全文/摘要、GUID 保持、绝对 URL、中文/emoji/XML 特殊字符及可见性排除。
- [x] HTTP 回归验证四个入口直接 200、正确 Content-Type、GET/HEAD 和 `/feed/` 与 `/rss.xml` 正文相同；冲突配置不得覆盖用户文件。

### C. 页面元数据、分页与 JSON-LD

参考文件：`internal/render/render.go`、`templates/page.html`、`static/assets/theme.css`、`static/assets/navigation.js`、`internal/render/render_test.go`，按需在现有 Node 测试约定中加聚焦导航回归。

- [x] 自动生成首页/文章/分类标签/分页元数据，增加页码显示与公开文章 JSON-LD。
- [x] 检查普通与窄屏分页标题，第一页/第二页/单页状态；保留整体主题风格。
- [x] 测试初始 HTML、正确 JSON 解析、真实日期和绝对地址；特殊字符和 `</script>` 不破坏 head。
- [x] 验证列表→文章→另一文章→列表导航的数据更新，不能残留旧 JSON-LD、OG、title 或 canonical。
- [x] 不修改任何既有博文的标题层级、目录或正文。

### D. 文档、整体回归与交付

参考文件：`docs/content-output.md`、`docs/theme.md`、`docs/configuration.md`，必要时更新现有示例说明；避免在每份文档重复完整设计。

- [x] 文档准确说明 feed 格式、全文默认行为和元数据来源，主入口仍为 rss.xml。
- [x] 保持 Go 1.27.3 或当前更高稳定补丁版的既有工具链要求与依赖锁定；不升级无关依赖。
- [x] 执行现有 Go test/vet、Node 浏览器脚本测试、Python import-preview/package-release 单元测试；对本次改动增加有意义的回归用例。
- [x] Windows 本机与可用 Linux 环境进行构建及共享 Handler/发行包验证。未实际执行的平台或检查明确列出，不用交叉编译冒充原生执行。
- [ ] 按本版 A 重新执行整体回归及本地构建产物验收；保留 B/C 已实现功能、原有统计、静态资源和配置行为。
- [ ] 复查 diff，确认无本轮遗留专用路由或真实部署数据；提供完成清单、验证命令/结果、可审查提交或分支、剩余限制。可以本地提交；不推送、合并、发布或部署。

本轮线上只读证据位于本地忽略目录 `output/crawl-audit-20261026/REPORT.md` 及相邻 JSON/HTTP 响应，供参考；这些证据不是公开打包内容。所有实施判断以当前源码为准。
