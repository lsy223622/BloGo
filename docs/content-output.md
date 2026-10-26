# 内容索引与页面身份

`SiteIndex.ByID` / `ByURL` 保留可直接访问的生产内容，包括 Unlisted。公开时间列表只包含 Public；预览模式可以额外显示 Draft，缺少日期的草稿排在有日期文章之后。

发布时间与更新时间分别排序：有值优先、日期倒序、稳定 ID 升序。独立页面单独保存，无日期页面仍按稳定 ID 排列。排序不读取文件时间。

分类聚合包含文章所属分类的祖先，同一分类按文章 ID 去重。分类导航、标签列表与归档只从 Public 文章产生；未使用的元数据项不会显示。分类直接链接到对应文章列表。展示名与父分类变化不改变分类/标签的 URL 和身份。

分类和标签配置在 `content/site.yaml` 的 categories/tags 映射中。文章的分类、标签均只保留与配置中稳定 ID 的交集，重复项去重。未配置的标签不会显示在文章下方，也不会生成标签页；已配置但未被 Public 文章使用的标签不进入公开标签列表。仅在 Unlisted 或 Draft 中使用的已配置标签，在文章下方显示为普通文字。

分类配置不影响文章生成和可见性。未填写分类，或填写的分类全部未配置时，文章按原有内容状态正常生成，不归入分类；填写多个分类时，只归入已配置的分类。修改分类配置会更新分类导航与文章列表，文章的 ID、URL、aliases 和统计身份保持不变。

每个列表第一页使用基础路径，后续页追加 `page/{n}/`；分页身份为 `{基础ID}:page:{n}`。分类使用 `category:{id}`，标签使用 `tag:{id}`，归档使用 `archive:{yyyy}` 或 `archive:{yyyy}-{mm}`。无内容时保留首页和系统总览，不生成空的后续页。

上一篇/下一篇按 Public 文章的发布时间建立，与更新时间列表分离。RSS 的 GUID 使用稳定内容 ID；RSS 和 Sitemap 排除 Unlisted/Draft，Sitemap 另排除 404。生成链接均使用规范 URL。

当前发布的重定向数据覆盖无尾斜杠、`index.html` 和历史 aliases；目标只能为内容的当前规范 URL。Go HTTP服务返回301并保留query，构建成功后规则随发布生效。

用户附件位于 `content/assets/`，映射到 `/media/`；展示信息、分类与标签统一定义于 `content/site.yaml`。页面扫描只读取 posts/pages，附件与站点设置分别校验并纳入一致的构建快照。

## 站点订阅

默认生成全文，保留摘要。`/rss.xml` 与 `/feed/` 是相同的RSS2；`/feed/rss/` 是RSS1/RDF，`/feed/atom/` 是Atom1。四个地址均直接返回200，媒体类型分别是application/rss+xml、application/rdf+xml、application/atom+xml，使用UTF-8。页面发现链接和页脚使用/rss.xml。

RSS2的description保存摘要，content:encoded保存完整HTML；RDF使用相应全文扩展，Atom提供text摘要和HTML内容。三种格式复用公开文章的顺序和正文转换，排除独立页面、Draft、Unlisted及构建时发布日期尚在未来的文章。RSS2 GUID继续使用原稳定ID；Atom和RDF条目标识由站点origin及稳定ID组成，修改slug不会更换身份。

正文只包含文章内容，链接和图片地址按文章URL解析为绝对地址。全文不注入主题外壳、统计或开发重载脚本；依赖JavaScript的图表保留可读源码。阅读器阅读不补记站内访问事件。文章更新时间取实际内容日期，缺省时Atom条目使用发布日期；没有文章的Atom feed以生成时间填写其必需的feed级updated。

## 别名通配符

Front Matter 的 `aliases` 可以填写精确别名和通配模式，目标都是所属内容的规范 URL。例如：

```yaml
aliases:
  - /legacy/example/
  - /legacy/example/replies-*
  - /old/*/entry-*/
```

`*` 匹配单个路径段内零个或多个字符，必须匹配整个路径。例如第二条匹配 `/legacy/example/replies-` 和 `/legacy/example/replies-27`，不会匹配额外的斜杠或子路径。通配模式保留填写时的尾斜杠：第三条要求末尾有 `/`。精确别名继续沿用原有规范化规则；仅填写精确别名不会自动产生任何通配规则。

正常页面、已存在的静态文件、系统入口和精确别名优先。通配模式之间先比较固定字符数量，多者优先；相同数量按模式字符串的 ASCII 升序选择。同一个模式不能指向不同内容。匹配后直接返回 301 到最终规范 URL，保留查询字符串；私有路径和路径穿越仍不能访问。

模式保存在现有 `meta/redirects.json` 中，随发布快照更新，由 BloGo 的 `serve` 和 `dev` 处理。`meta/redirects.conf` 保留精确映射，通配规则需由 BloGo Handler 服务，以保证页面、静态文件及私有路径的优先级。
