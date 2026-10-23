# Go 极简混合渲染博客：技术设计

-   文档版本：1.2
-   适用范围：V1
-   状态：V1 实现与本地 Gate E 验收完成；生产环境配置由部署方核对
-   配套文档：[`IMPLEMENTATION_PLAN.md`](IMPLEMENTATION_PLAN.md)

## 1. 文档目的与规范优先级

本文定义系统的架构、数据模型、模块职责、处理流程、持久化格式和对外契约。实现、测试和部署均应遵循本文。

`IMPLEMENTATION_PLAN.md`
负责安排任务顺序、交付物和验收工作，不应另行定义与本文冲突的数据语义。发现冲突时，先更新设计文档，再调整实施计划和测试。

本文中的要求分为三类：

-   **确定**：已由项目需求明确决定，实施必须遵守。
-   **推荐默认值**：V1
    采用该值；只有在实际验证发现问题后，才通过修改本文来调整。
-   **待实现时验证**：需要用真实文章、依赖库或部署环境验证，但不应因此提前引入额外架构。

## 2. 项目目标与非目标

### 2.1 目标

构建一个面向单作者的 Go 博客系统：

1.  Markdown/HTML 文件是内容权威源。
2.  Go 程序将内容完整生成静态 HTML，由 Nginx 提供。
3.  以少量 JavaScript 增强灯箱、主题切换、统计采集等体验；基本阅读不依赖
    JavaScript。
4.  独立 Go 统计服务接收浏览器上报的页面访问事件，并使用 SQLite
    聚合统计。
5.  通过定期全目录内容指纹扫描发现更新，构建成功后切换发布版本。
6.  公开站点、构建器、统计数据库和私有统计报告具有明确的文件权限边界。
7.  代码和配置尽可能简单，避免通用 CMS、插件系统或可替换后端抽象。

### 2.2 明确不做

V1 不实现：

-   在线文章编辑、管理后台、注册登录、多作者或评论系统。
-   文章内容数据库、通用插件系统、通用主题市场。
-   密码保护文章或服务器端文章访问授权。
-   站内全文搜索、作者归档、定时发布。
-   完整 Typecho 自动迁移工具。
-   统计用户跨日追踪、浏览器指纹、完整原始访问日志分析。
-   通用 SSG 框架、分布式构建、消息队列、Redis 或多实例写入。
-   为 CDN 设计的专门统计采集机制。

## 3. 已确定的产品与技术决策

  项目        V1 决策
  ----------- --------------------------------------------------
  语言        Go
  内容来源    Markdown 和带元数据的 HTML 文件
  元数据      YAML Front Matter
  HTML 模板   Go `html/template`
  Markdown    Goldmark；具体扩展在内容模块实现时验证并记录
  文章身份    可选独立 `id`；缺省使用解析后的 slug
  摘要        显式摘要优先，否则从正文自动提取
  日期        文章使用显式日期；独立页面可无日期，无日期项按稳定 ID 排序
  排序        同时建立按发布时间和更新时间排序的索引
  文章状态    `public`、`unlisted`、`draft`
  密码保护    不实现
  特殊页面    支持完整 HTML 文档，不强制套用普通文章模板
  内容更新    定期全目录指纹扫描；默认每 60 秒
  构建        完整构建，不做增量依赖图
  发布        构建到独立版本目录，成功后原子切换当前版本
  统计        浏览器 Beacon、Go HTTP 服务、SQLite
  UV          按站点时区日期轮换的随机浏览器标识，属于近似估算
  私有报告    定期生成静态 HTML，存放在公网目录之外
  CDN         V1 不考虑
  Git         可选的外部内容更新机制；构建器不依赖 Git

生产部署目标为 Linux 本地文件系统、Nginx 和 systemd。Windows 支持内容解析、静态构建、统计开发和本地预览；生产发布的符号链接、原子替换与权限验收在 Linux 上执行。

## 4. 部署架构与信任边界

### 4.1 逻辑组件

1.  **内容源**：文章、独立页面、静态资源、模板和配置。
2.  **构建器 /
    watcher**：读取内容源，建立索引，渲染网站，校验结果，发布版本。
3.  **Nginx**：提供当前版本中的静态文件，并把 `/api/events`
    反向代理到本机统计服务。
4.  **统计服务**：接收事件、验证页面身份、去重、更新 SQLite。
5.  **报告任务**：从 SQLite 生成公开 `stats.json` 和私有 `report.html`。
6.  **可选 Git 更新任务**：将仓库更新到内容工作目录；与构建器分离。

### 4.2 进程和权限

-   Nginx 只读取公开站点文件和单独映射的公开统计 JSON。
-   构建器读取内容、模板、配置，并写入版本目录；不需要读取统计数据库、统计密钥或
    Git 凭据。
-   统计服务读取当前版本的内部 Manifest，写入统计数据库；不读取 Markdown
    源文件，也不写入博客 HTML。
-   报告任务读取统计数据库，并只写入统计公开输出目录和私有报告目录。
-   Git
    更新任务只负责更新内容工作目录，不负责发布，也不需要访问统计数据库。
-   所有应用进程均以非 root 用户运行。
-   统计服务只监听 loopback 地址，由 Nginx 代理。
-   私有报告与数据库必须位于 Nginx 不会公开映射的目录。

### 4.3 建议的服务器目录

``` text
/srv/blog/
├── source/                         # 内容工作目录
├── releases/
│   ├── <release-id>/
│   │   ├── public/                 # Nginx 可读
│   │   └── meta/
│   │       ├── manifest.json       # 内部页面清单
│   │       └── fingerprint
│   └── ...
├── current -> releases/<release-id>
└── staging/                        # 未发布的临时构建

/var/lib/blog-stats/
├── stats.db
├── stats.db-wal
├── stats.db-shm
└── visitor-hmac.key

/srv/blog-stats-public/
└── stats.json

/var/lib/blog-stats-private/
└── report.html
```

Nginx 的网站根目录指向 `/srv/blog/current/public`。公开统计 JSON
通过精确的 `location` / `alias` 映射到
`/srv/blog-stats-public/stats.json`。不要把 `/var/lib/blog-stats/`
整个目录映射到公网。

上述路径是推荐部署布局，不是硬编码常量；实际路径通过配置和部署文件指定。

## 5. 项目代码结构

``` text
blog/
├── cmd/blog/main.go
├── internal/
│   ├── config/
│   ├── content/
│   ├── markdown/
│   ├── site/
│   ├── render/
│   ├── build/
│   ├── preview/
│   └── stats/
├── content/
│   ├── posts/
│   └── pages/
├── templates/
├── static/
├── taxonomies.yaml
├── config.yaml
├── go.mod
├── go.sum
└── docs/design/
    ├── DESIGN.md
    └── IMPLEMENTATION_PLAN.md
```

模块含义：

-   `config`：配置解析、默认值和配置校验。
-   `content`：Front Matter、源文件类型、内容模型和可见性规则。
-   `markdown`：Markdown AST、HTML、摘要、目录和标题锚点。
-   `site`：全站索引、分类、标签、归档、排序和导航关系。
-   `render`：模板和页面输出。
-   `build`：扫描、快照、校验、版本目录与发布。
-   `preview`：本地预览和热重载。
-   `stats`：SQLite、事件处理、聚合、统计 JSON 和报告。

这些是代码职责边界，不是插件接口。优先使用普通函数和少量结构体；只有确实存在多个实现时才引入
Go interface。

## 6. 配置契约

配置文件采用 YAML，拒绝重复键和未知的结构字段。先完成类型、格式及路径关系校验，再按命令校验必需字段和文件访问条件；不能让缺失字段在操作中途才造成含糊错误。

配置字段（必需性按下方命令表解释）：

  字段                        含义                          默认/约束
  --------------------------- ----------------------------- ----------------------------------------
  `site.base_url`             规范站点 URL                  站点命令必填；HTTP(S) origin，不含凭据、query、fragment 或子路径
  `site.title`                站点名称                      构建与预览必填
  `site.timezone`             站点时区                      默认 `Asia/Shanghai`，使用 IANA 时区名
  `site.language`             HTML / Feed 语言              默认 `zh-CN`
  `paths.source`              内容源目录                    构建与预览必填
  `paths.templates`           模板目录                      构建与预览必填
  `paths.static`              公共静态资源源目录            构建与预览必填
  `paths.taxonomies`          分类与标签元数据文件          默认 `taxonomies.yaml`
  `paths.releases`            版本目录                      发布命令必填
  `paths.staging`             暂存目录                      发布命令必填，必须与发布目录分离
  `paths.current`             当前版本符号链接              发布命令必填
  `build.scan_interval`       watcher 扫描间隔              默认 60 秒
  `build.summary_max_chars`   自动摘要最大 Unicode 字符数   默认 160
  `build.page_size`           文章列表每页数量              默认 10
  `build.strict_links`        缺失内部内容链接是否失败      默认 `false`
  `build.keep_releases`       含当前版本的回滚版本保留数    默认 3，正整数
  `stats.listen_addr`         统计服务监听地址              默认 `127.0.0.1:8787`；必须为回环 IP 与有效端口
  `stats.manifest_path`       当前生产 Manifest 路径        统计服务和报告命令必填
  `stats.database_path`       SQLite 文件路径               统计服务和报告命令必填
  `stats.visitor_key_path`    HMAC 密钥路径                 统计服务必填，持久化且权限受限
  `stats.public_output`       公开统计 JSON 路径            报告命令必填
  `stats.private_output`      私有报告 HTML 路径            报告命令必填
  `stats.event_retention`     事件去重记录保留时间          默认 72 小时
  `stats.visitor_retention`   每日访客去重记录保留时间      默认 7 天
  `stats.report_interval`     报告刷新周期                  默认 5 分钟
  `extra`                     站点模板扩展数据              可选映射

以上 YAML 键名是 V1 配置契约。所有相对路径相对于配置文件所在目录解析，而不是进程的当前工作目录。

路径校验必须拒绝源目录、暂存目录和发布目录的危险重叠；统计数据库和私有报告不得位于公开输出目录内。

命令及其访问边界：

| 命令 | 必需配置与行为 |
| --- | --- |
| `blog build --output <dir>` | 站点、内容、模板、静态资源、分类标签及构建配置；输出目录包含 public/ 和 meta/，不切换生产版本 |
| `blog publish` | 构建配置加 releases、staging、current；构建并发布一次，可对相同输入主动重建 |
| `blog watch` | 与 publish 相同，加扫描间隔；持续扫描并发布 |
| `blog dev` | 构建配置；使用独立开发输出及回环 HTTP 端口，预览草稿 |
| `blog stats` | 站点时区、base URL、Manifest、监听地址、数据库和密钥；不打开内容和模板 |
| `blog report` | 站点时区、base URL、Manifest、已有数据库和两个报告输出路径；不需要访客密钥 |

所有输出路径都校验与输入的重叠关系。发布用 staging、releases 及 current 所在目录必须处于同一文件系统。配置中的其他组件路径即使存在，也不意味着当前命令需要读取其文件；静态构建和预览不要求统计配置。

内容配置、Front Matter 和分类标签文件各自提供 `extra` 映射保存模板扩展数据；保留字段遵守固定类型，新增正式字段时更新文档与 schema 版本，不通过静默接受拼写错误来扩展。

## 7. 内容源与 Front Matter

### 7.1 文件类型与来源目录

-   `content/posts/`：文章 Markdown 文件。
-   `content/pages/`：独立页面，可使用 Markdown 或带 Front Matter 的
    HTML 文件。
-   `templates/`：Go HTML 模板。
-   `static/`：复制到公开站点根路径的静态资源。
-   `config.yaml`：站点配置。
-   `taxonomies.yaml`：分类和标签的稳定 ID、展示名及分类层级。

V1 的静态资源采用统一的 `static/` 根目录，并通过站点根路径引用，例如
`/images/cover.webp`。构建器校验站内资源引用是否存在。文章附件也放入该目录的合适子目录，不把内容源目录暴露给
Nginx。

输入扫描只处理明确允许的普通文件类型；默认拒绝符号链接、设备文件和越出允许目录的路径。

### 7.2 Front Matter 字段

示例：

``` yaml
---
id: nanoink-design
title: "NanoInk 的硬件设计"
slug: nanoink-hardware
published: 2026-10-15T10:00:00+08:00
updated: 2026-10-22T13:30:00+08:00
status: public
layout: article
categories:
  - hardware
tags:
  - esp32
  - pcb
summary: "介绍 NanoInk 的硬件架构与设计思路。"
stats: true
extra:
  cover: /images/nanoink.webp
---
```

字段契约：

  字段           类型           规则
  -------------- -------------- -----------------------------------------------
  `id`           string         可选；缺省使用最终解析的 slug；遵守稳定 ID 语法
  `title`        string         发布内容必填
  `slug`         string         可选；缺省从源文件名生成
  `permalink`    string         可选；覆盖默认页面 URL，规范形式见 7.4
  `aliases`      string array   可选；旧 URL，生成重定向配置数据
  `published`    日期/时间      Public / Unlisted 文章必填；独立页面和 Draft 可缺省
  `updated`      日期/时间      可选；存在 `published` 时缺省等于它，否则保持无值
  `status`       enum           `public`、`unlisted`、`draft`；缺省为 `draft`
  `layout`       enum           `article`、`page`、`standalone`；默认值与组合见 8.3
  `categories`   string array   可选；引用分类元数据中的稳定 ID，重复 ID 去重
  `tags`         string array   可选；引用标签元数据中的稳定 ID，重复 ID 去重
  `summary`      string         可选；显式值优先
  `stats`        bool           可选，默认 `true`；控制该页采集脚本和事件接收
  `extra`        map            可选；普通数据，不允许执行表达式

以上为 V1 全部 Front Matter 保留字段。未知字段与类型错误失败；扩展数据放入 `extra`，不执行模板表达式。`title`、`summary` 和展示文本允许 Unicode，按普通模板数据转义。

文章需要有效的 `published` 才能进入任何生产时间索引。Draft 缺少
`published`
时仍可本地预览，但在日期排序列表中放在有日期的文章之后，再按稳定 ID
排序。独立页面不进入文章时间索引；模板需要按日期列出独立页面时，有目标日期的页面倒序在前，无目标日期的页面在后，再按稳定 ID 升序排列。

日期时间允许 ISO 8601 / RFC 3339 时间戳和纯日期。纯日期按
`site.timezone` 解释为当天 00:00:00。不得使用文件系统时间、Git
提交时间或首次发现时间替代元数据日期。`updated` 缺省时等于
`published`；两者都有值且 `updated < published` 时构建失败。独立页面允许仅提供 `updated`，或两个日期都省略；省略值不替换为系统时间。

### 7.3 状态语义

-   **Public**：生成页面；可进入首页、分类、标签、归档、Feed、Sitemap、相关推荐、公开统计排行。
-   **Unlisted**：生成可直接访问的静态页面；不进入公开列表、Feed、Sitemap、相关推荐和公开热门排行；添加
    `noindex`。它不是保密机制。
-   **Draft**：生产构建中完全不输出；本地预览可见。

Unlisted 页面和其静态附件只要知道 URL 就可以访问。V1
不提供密码保护、身份验证或私密文件保护。

### 7.4 ID、slug 和 URL

-   内部内容 ID 在文章和页面类型中加命名空间，例如
    `post:nanoink-design`、`page:about`。
-   显式 `id` 在改标题或改 slug 后保持稳定。
-   未填写 `id` 时使用 slug；修改 slug 也会改变默认身份。
-   显式 `id`、slug、分类 ID 和标签 ID 使用同一 ASCII 语法：`[a-z0-9]+(?:-[a-z0-9]+)*`。完整的内部页面 ID 可包含由系统生成的命名空间分隔符。
-   默认文章 URL 为 `/posts/{slug}/`；独立页面默认 `/{slug}/`。独立页面保留源文件的相对父目录，例如 `pages/guide/part-two.md` 对应 `/guide/part-two/`；父目录段也遵守 slug 语法，不合规则要求显式 permalink。
-   `permalink` 必须是站点根路径下的规范路径，不含
    query、fragment、控制字符或路径穿越片段。
-   URL 全站唯一，任何冲突都使构建失败。
-   `aliases` 不生成虚假的 HTML 重定向页；构建器验证后导出 Nginx
    重定向片段，由管理员审核应用。
-   V1 不承诺完整 Typecho URL 兼容。

默认 slug 采用小写 ASCII
字母、数字和连字符的安全形式。若文件名无法生成有效
slug，要求作者显式填写 `slug`，不做不可靠的自动转写。具体 slug
规范化规则为：去掉最后的文件扩展名、ASCII 字母转小写、空格和下划线转连字符、合并连续连字符并去掉首尾连字符；剩余文本不匹配上述语法时失败。显式 ID 和 slug 必须已经符合语法，不静默改写。

#### 7.4.1 规范路由与尾斜杠

HTML 页面采用目录式 URL，根路径为 `/`，其他规范 URL 均以 `/` 结尾，对应输出 `{path}/index.html`。permalink 的非根路径段也采用上述 ASCII 语法；缺少尾斜杠时补上，拒绝 `.html` 后缀、百分号编码、反斜杠、空路径段及 `.` / `..`。静态文件和 API 使用各自的文件或接口 URL。

| 页面 | 规范 URL |
| --- | --- |
| 首页及第 n 页 | `/`、`/page/{n}/`，n 从 2 开始 |
| 文章 | `/posts/{slug}/` |
| 独立页面 | `/{slug}/` 或带父目录的路径，支持 permalink 覆盖 |
| 分类总览及单分类 | `/categories/`、`/categories/{id}/` |
| 标签总览及单标签 | `/tags/`、`/tags/{id}/` |
| 归档总览、年份和月份 | `/archives/`、`/archives/{yyyy}/`、`/archives/{yyyy}/{mm}/` |
| 最近更新 | `/updates/` |
| 404 页面 | `/404/`，由 Nginx 保持 404 状态返回 |

可分页列表的后续页在其基础路径后追加 `page/{n}/`。第一页只使用基础 URL；页码和年月段由系统生成。已存在页面的无尾斜杠 URL 以 301 跳转到带尾斜杠形式，保留 query；生产 Nginx 与本地预览行为一致。canonical、内部生成链接、Feed、Sitemap 和 Manifest 都使用规范 URL，页面重定向响应不采集 PV。直接访问页面的 `index.html` 时也 301 到其目录 URL。

系统路由、内容路由、aliases 和静态文件共用输出冲突检查。`/api/`、`/data/`、`/assets/` 为保留前缀，以上系统页面路由、分页路由以及 `/rss.xml`、`/sitemap.xml`、`/robots.txt` 为保留输出；内容不能占用。static 文件可以使用 `/assets/`，但不能覆盖生成文件或占用 `/api/`、`/data/` 的服务映射。历史 aliases 可以保留 `.html` 等旧形式，路径区分大小写；须独立进行安全校验、冲突检测。Nginx map 规则使用经转义并首尾锚定的大小写敏感正则，目标只能是对应内容的当前规范 URL。

路由选择参考 [Hugo URL management](https://gohugo.io/content-management/urls/)、[Ghost routing](https://docs.ghost.org/themes/routing) 和 [Hexo permalinks](https://hexo.io/docs/permalinks)。[WordPress](https://developer.wordpress.org/reference/functions/user_trailingslashit/) 会依据配置添加或去除尾斜杠，并无所有博客必须一致的规则；本项目固定一种形式，减少重复 URL 与部署规则。

### 7.5 分类和标签

分类与标签使用独立的 `taxonomies.yaml`，由 `paths.taxonomies` 指定，参与输入指纹和快照：

``` yaml
schema_version: 1
categories:
  tech:
    name: "技术 / Tech 🛠️"
    description: "技术文章"
  hardware:
    name: "硬件 & PCB"
    parent: tech
    extra: {}
tags:
  esp32:
    name: "ESP32 ⚡"
  pcb:
    name: "PCB / 电路板"
extra: {}
```

-   顶层保留字段为 `schema_version`、`categories`、`tags`、`extra`；schema_version 必填且为 `1`，两个集合可省略或为空映射。分类项为 `name`、`description`、`parent`、`extra`，标签项为 `name`、`description`、`extra`；条目内 `name` 必填，其余项可省略。
-   映射键是稳定 ID，遵守 7.4 的 ASCII 语法；文章引用 ID，不从展示名自动生成身份。分类和标签各有自己的 ID 命名空间。
-   展示名允许正常 Unicode 文本，包括大小写英文、中文、数字、标点和 emoji；拒绝空白名称及 NUL、换行等控制字符，保留原有显示形式。不按展示名大小写或 Unicode 形式自动合并条目。
-   分类层级通过 `parent` 引用另一个分类 ID；拒绝未知父分类、自引用和环。分类 URL 使用自己的稳定 ID，不因改展示名或父分类而改变。
-   每篇文章可属于多个分类；归入子分类的文章也进入祖先分类索引，同一页面内按文章 ID 去重。
-   内容分类、标签只保留与配置中稳定 ID 的交集。文章未填写分类时正常生成；填写了分类但全部未配置时，在生产和预览中均排除，且不占用 ID、URL 或 aliases。多个分类中至少一个已配置时保留文章，仅归入已配置分类。未配置标签不展示；已配置但未使用的分类、标签不进入总览。总览与归档仅根据 Public 文章使用的条目及其分类祖先生成，避免通过分类标签暴露 Unlisted 内容。修改展示名保留 URL 和统计身份。
-   YAML 重复键、未知结构字段和错误类型失败，扩展数据放入 `extra`。

### 7.6 自定义字段

`extra` 只用于模板读取的普通 YAML
数据，例如字符串、数字、布尔值、列表和映射。它不是模板表达式、脚本入口或文件访问接口。字段的含义由使用它的模板约定，不需要通用字段注册系统。

## 8. Markdown、摘要和页面渲染

### 8.1 Markdown

采用 Goldmark。V1 支持范围在 CNT-04 中通过真实示例文章验证，并形成
`docs/markdown-support.md`。至少验证标题、段落、链接、图片、引用、列表、代码围栏、表格、任务列表、脚注、删除线、原生
HTML 和数学公式。

原生 HTML
被视为作者可信内容。系统不试图对作者自己的页面实现完整安全沙箱，但不能让
Markdown 或模板数据在构建阶段获得任意文件系统访问或命令执行能力。

### 8.2 摘要

摘要规则：

1.  Front Matter 的非空 `summary` 优先。
2.  否则从 Markdown AST 的正文文本提取。
3.  忽略代码块、脚本、样式、HTML 标签和数学公式。
4.  优先取首个有意义的正文段落，再按 `build.summary_max_chars` 截断。
5.  按 Unicode 字符截断，不拆分 UTF-8 编码字符；移除多余空白。
6.  同一输入必须产生确定的摘要。

自动摘要不直接从渲染后的 HTML 通过正则删除标签获得。

以上自动提取规则适用于 Markdown；HTML 页面通过显式 `summary` 提供摘要，缺省保持空值。

### 8.3 布局

-   `article`：Markdown 渲染后套用普通文章模板。
-   `page`：独立页面，可采用 Markdown 或页面模板。
-   `standalone`：源文件是完整 HTML 文档；移除 Front Matter
    后保留作者的文档结构，不强制套用站点模板。

布局默认值和允许组合：`posts/*.md` 默认且只允许 `article`；`pages/*.md` 默认 `page`，也允许 `article` 布局但仍属于独立页面；`pages/*.html` 默认 `standalone`，也允许 `page`，此时正文作为可信 HTML 片段嵌入页面模板。Markdown 不允许 `standalone`。

完整 HTML 页面的作者负责自己的 `<head>`、元数据、DOM、CSS 和
JavaScript。若页面启用统计，构建器在有效的 `</body>`
前插入带 Manifest 页面 ID 的统计脚本；若无法安全确定插入位置，构建失败，作者可设置 `stats: false`。使用 HTML tokenizer 识别实际结束标签，不以字符串搜索匹配脚本或注释中的 `</body>`；保留其他源文本。Unlisted Standalone 页面必须由作者在 head 提供有效的 `noindex` 元数据，构建器校验它；普通布局由模板生成 `noindex`。

资源指纹与压缩仅处理构建器管理的 CSS / JS 和普通模板输出。作者的 HTML 片段、Standalone 文档、内联脚本与样式保留原文，不做全站字符串替换；显式引用的静态资源路径保持可用。数学公式采用自托管 KaTeX 客户端渲染时，禁用 JavaScript 仍应能读到公式源码和基本正文。

普通模板使用 `html/template`，元数据默认经过上下文转义。Markdown
生成的正文 HTML 只能通过明确的可信 HTML
入口插入模板。作者控制的脚本与站点同源，不视为隔离沙箱。

### 8.4 阅读功能与 SEO

V1 包括：

-   响应式布局、自动深浅色和手动切换。
-   代码语法高亮，优先构建时处理。
-   标题锚点和文章目录。
-   图片灯箱。
-   数学公式，默认优先验证 KaTeX。
-   友情链接和侧边栏组件。
-   SEO title、description、canonical、Open Graph。
-   RSS 2.0、Sitemap、robots.txt、404 页面。

基本正文、导航和 Feed 不依赖 JavaScript。客户端 JavaScript
用于统计、灯箱、主题切换及特殊页面交互。

### 8.5 默认主题视觉

默认主题参考 [Jasmine 2.5.5](https://github.com/liaocp666/Jasmine) 及作者此前使用的修订版，使用本设计定义的页面和功能组织界面。

-   桌面沿用窄左导航、中央阅读区、右侧信息栏的布局关系；参考 Jasmine 的 12 栏网格中 1 / 8 / 3 的比例、中央细分隔线和宽松留白。
-   首页以清晰的文章标题、短摘要、分类与日期组成列表；有封面时在文字旁显示缩略图。文章页以正文排版为主，目录、代码块与图片样式协调。
-   浅色以白色、浅灰和低饱和强调色为主；深色使用分层的深灰背景、浅色正文和足够对比度的链接。通过少量 CSS 变量统一配色，主题切换沿用相同视觉层级。
-   手机采用单列，侧栏内容合理折叠，导航、分页和主题切换可直接操作，长标题、Unicode 标签和图片不造成横向溢出。
-   友情链接延续修订版的固定头像尺寸与长名称换行处理。字体优先使用本地或自托管资源；基本阅读保留系统字体链，页面允许用户缩放。

REN-01 / REN-02 以首页、文章、分类标签、归档和友链样例进行桌面及手机、浅色及深色视觉核对。若直接复用主题代码或资源，保留其适用的许可与署名。

## 9. SiteIndex 与排序

解析后的内容进入一个内存中的 `SiteIndex`，至少包含：

-   `ByID`：内部 ID 到内容的映射。
-   `ByURL`：规范 URL 到内容的映射。
-   `ByPublished`：按发布时间排序的文章列表。
-   `ByUpdated`：按更新时间排序的文章列表。
-   `ByCategory`、`ByTag`、`ByMonth`。
-   Public 文章集合。

所有日期排序均使用 `(有目标日期优先, 目标日期倒序, 稳定 ID 升序)`。有发布时间时，更新时间缺省等于发布时间。没有发布日期的 Draft 不参与公开索引；本地预览时放在有日期的文章之后。独立页面单独保存，模板按日期列出时也遵守此规则；无日期顺序不依赖扫描先后或文件系统时间。

默认首页、分类、标签、归档、RSS
和上一篇/下一篇按发布时间排序。最近更新列表使用更新时间排序。构建时可以计算两套导航关系，但不需要为每套顺序重复保存文章数据。

Unlisted 和 Draft
不进入任何公开聚合。独立页面不参与文章上一篇/下一篇导航，除非未来明确修改此规则。

## 10. 构建输入、快照与状态机

### 10.1 完整构建

V1 每次发现输入变化都完整重建站点，不做增量依赖图。

参与指纹的输入包括：

-   所有被允许的内容文件、模板、静态资源、分类标签元数据和配置文件的相对路径及内容摘要。
-   构建器版本标识。
-   影响输出的显式构建选项。

路径应按稳定顺序排列后再计算整体
SHA-256。文件修改时间只能作为扫描优化线索，不能作为内容语义或唯一变化依据。

### 10.2 状态机

``` text
IDLE
  -> SCANNING
  -> SNAPSHOT
  -> BUILDING
  -> VALIDATING
  -> PUBLISHING
  -> IDLE
```

-   `IDLE`：等待下一次扫描。
-   `SCANNING`：计算输入指纹。
-   `SNAPSHOT`：将输入复制到独立快照，确认快照与扫描指纹一致。
-   `BUILDING`：只从快照读取并完整生成站点。
-   `VALIDATING`：检查元数据、路由、链接、资源、可见性和 Manifest。
-   `PUBLISHING`：将校验成功的暂存结果发布为新版本。
-   任何确定性构建错误都保持旧发布版本不变，并记录错误。

同一生产目标通过进程间排他锁只允许一个 publish / watch 任务发布，竞争者明确失败；独立输出目录的 build / dev 不占生产锁。锁随进程退出释放。构建输出不进入输入扫描范围。

### 10.3 一致性

1.  扫描得到指纹 `F0`。
2.  创建快照，验证快照指纹等于 `F0`。
3.  只从快照进行构建。
4.  构建并校验完整版本。
5.  校验通过后发布，记录本次快照对应的 `F0`。
6.  下一轮扫描发现不同指纹时再构建；复制期间快照与 `F0` 不符则放弃，后续重试。

一次构建只读取同一个快照。内容编辑或外部更新应尽量在扫描前完成；若产生有效但不符合作者预期的内容组合，作者修正后重新发布。构建器版本变化也使输入指纹变化。

### 10.4 校验策略

必须失败：

-   Front Matter 结构错误或必填字段缺失。
-   重复 ID、重复 URL、非法 permalink。
-   Public / Unlisted 文章缺少有效发布日期。
-   无效日期关系、非法状态或布局。
-   Draft 被输出到生产版本。
-   Unlisted 被写入公开索引、Feed、Sitemap 或公开热门排行。
-   内部输出路径冲突、路径越界或不允许的文件类型。
-   关键本地资源缺失。
-   Manifest 与生成页面集合不一致。

默认警告：

-   文章内部链接指向不存在的内容（可在配置中提升为失败）。
-   外部链接无法访问。
-   不影响发布正确性的非关键 SEO 提示。

不在每次生产构建时强制访问所有外部链接，避免网络波动阻断发布。

## 11. 发布版本与 Manifest

### 11.1 版本布局

每个版本同时保存公开站点和内部元数据：

``` text
releases/<release-id>/
├── public/
└── meta/
    ├── manifest.json
    └── fingerprint
```

`current` 符号链接指向整个 release 根目录。Nginx 根目录是
`current/public`；统计服务只读 `current/meta/manifest.json`。这样一次
`current` 指针切换同时确定公开 HTML 和对应 Manifest 的版本。

发布时创建新符号链接并在同一文件系统内通过原子重命名替换
`current`。不能原地覆盖当前公开目录。按 `build.keep_releases` 保留完整版本用于人工回滚；清理在切换成功后进行，始终保留当前版本。人工回滚前停止 watcher，确认内容源与所选版本一致后再恢复自动发布。

Nginx 只从 current 提供页面与资源，HTML 使用需重新验证的缓存策略，带内容哈希的资源可长期缓存。回滚版本的文件由 release 目录保存。


### 11.2 Manifest 契约

Manifest 是内部文件，不在 Nginx 公开路径中。逻辑结构：

``` json
{
  "schema_version": 1,
  "build_id": "build-identifier",
  "fingerprint": "sha256-hex",
  "generated_at": "2026-10-22T12:00:00Z",
  "pages": [
    {
      "id": "post:nanoink-design",
      "kind": "post",
      "url": "/posts/nanoink-hardware/",
      "title": "NanoInk 的硬件设计",
      "visibility": "public",
      "stats_enabled": true,
      "published": "2026-10-15T02:00:00Z"
    }
  ]
}
```

规则：

-   Manifest 列出当前生产版本的全部 HTML 页面：内容页、系统页面、分类标签页、归档页、分页及 404；内容状态为 Public 或 Unlisted，不包含 Draft。静态附件、Feed、Sitemap、JSON 与重定向响应不属于 HTML 页面。
-   `id` 是统计用稳定页面 ID。
-   `kind` 为 `post`、`page`、`system`、`category`、`tag`、`archive` 或 `pagination`；系统生成页面默认 Public，404 添加 `noindex` 且不进入 Sitemap。
-   `stats_enabled` 为布尔值，默认 `true`；内容页来自 Front Matter，系统生成页面为 `true`。禁用统计的页面仍在 Manifest 中，但不注入脚本且拒绝事件。
-   无日期页面省略 `published`，不得伪造日期。每个 HTML 输出恰好对应一个 Manifest 条目。
-   统计服务使用 `id`、`visibility`、`stats_enabled` 和规范 `url` 校验事件及过滤公开报告。
-   Manifest 中的 `title` 只供报告显示；统计服务不读取 Markdown。
-   当前版本变化时统计服务重新加载 Manifest。
-   删除页面后，旧统计历史保留在 SQLite；私有报告以历史页面 ID 标记为“已删除”。
-   已删除页面的 ID 不再接受新统计事件。
-   Public 改为 Unlisted 时仍允许事件写入，在下一次成功生成公开报告时排除它。
-   同一个 release 内的静态站点和 Manifest 必须一起发布。

系统生成页面的稳定身份如下，身份与展示名分离：

| 页面 | 稳定 ID |
| --- | --- |
| 首页、分类总览、标签总览、归档总览、最近更新、404 | `home`、`categories`、`tags`、`archives`、`updates`、`not-found` |
| 文章、独立页面 | `post:{内容id}`、`page:{内容id}` |
| 单分类、单标签 | `category:{分类id}`、`tag:{标签id}` |
| 年份、月份归档 | `archive:{yyyy}`、`archive:{yyyy}-{mm}` |
| 任意列表的第 n 页，n ≥ 2 | `{基础页面id}:page:{n}`，例如 `home:page:2`、`category:hardware:page:2` |

404 页的所有错误访问聚合到 `not-found`，不记录任意请求路径。页面标题、分类展示名或内容 permalink 改变不改变显式稳定 ID；分页按位置标识，文章移动到其他分页后不迁移旧分页的统计。

## 12. 访问统计语义

### 12.1 指标

-   **PV**：被服务端接受且去重后的页面浏览事件数量。
-   **UV**：按站点时区的自然日计算，使用当天不同访客标识估算。
-   **页面 PV**：按稳定页面 ID 聚合。
-   **来访来源**：由客户端从导航来源 `document.referrer` 提取的规范化主机名，或 `direct` / `internal`。
-   **当前页面**：事件的 `page_id`，通过 Manifest 对应规范 URL、标题和页面种类；每个 HTML 页面单独统计。
-   **热门文章**：指定时间窗内 Public 文章的页面 PV 排名。
-   **累计浏览量**：历史 `daily_page_stats.pv` 求和。

这些都是近似分析指标，不是防篡改计量。公开访客可以伪造请求、清理存储或更换浏览器，系统不承诺防止所有刷量。

### 12.2 浏览器标识

-   每次页面加载生成新的随机 `event_id`。
-   浏览器使用 `localStorage` 保存按站点时区日期轮换的随机
    `visitor_id`。
-   访客 ID 不跨日复用。
-   不读取浏览器指纹，不主动采集完整 IP、User-Agent、完整访问 URL
    或无关信息。
-   如果浏览器存储不可用，退化为当前页面加载的临时 ID，UV 可能偏高。
-   服务端只存储由持久化密钥计算的每日 HMAC 摘要，不保存客户端原始访客
    ID。
-   隐私说明应告知访客使用了当日随机标识；是否需要额外同意应根据实际适用法律与部署地区判断。

客户端计算日期和服务端分组必须使用同一个
`site.timezone`。客户端不提交可信日期，服务端根据自身时钟和配置时区确定事件日期。

### 12.3 来访来源与当前页面

区分导航来源与采集请求来源：

-   **导航来源**是进入当前页之前的页面。采集脚本读取 `document.referrer`，仅对 HTTP(S) 来源提取 `URL.hostname`，提交 `referrer_host`；不发送路径、端口、query 或 fragment。来源缺失、被浏览器隐藏或无法解析时提交空字符串。
-   **采集请求的 HTTP `Referer`** 通常指向发出 Beacon 的当前博客页，不能作为导航来源。服务端仅将其用于辅助检查同源请求及与 Manifest 当前 URL 的关系，不持久化该头；缺失时不因此拒绝有效事件。
-   **正在访问的页面**由 `page_id` 表达，服务端从当前 Manifest 解析其规范 URL 和种类。客户端不提交完整当前 URL，URL 改变后显式 ID 的历史 PV 仍累计在同一页面。

服务端要求 `referrer_host` 为长度不超过 253 的合法主机名或 IP 字面值；统一为小写 ASCII / IDNA 形式，拒绝 URL、凭据、端口和路径。空值记为 `direct`，与站点主机名相同记为 `internal`，其余按来源主机名聚合。同一次事件同时增加页面 PV 和该页面对应来源的 PV，私有报告可以分别查询来源总量及某页的来源分布。

这两类来源信息均非认证凭据，客户端字段也可伪造；不根据它们发起服务器端 HTTP 请求，不保存完整来源 URL。行为依据见 [HTML document.referrer](https://html.spec.whatwg.org/multipage/dom.html#dom-document-referrer) 与 [Referrer Policy](https://w3c.github.io/webappsec-referrer-policy/#determine-requests-referrer)。

## 13. SQLite 数据库契约

V1 只使用一个 SQLite 数据库，不存储文章内容，不保存完整原始访问事件。

### 13.1 DDL

``` sql
CREATE TABLE daily_site_stats (
    day TEXT PRIMARY KEY,
    pv INTEGER NOT NULL DEFAULT 0 CHECK (pv >= 0),
    uv INTEGER NOT NULL DEFAULT 0 CHECK (uv >= 0)
);

CREATE TABLE daily_page_stats (
    day TEXT NOT NULL,
    page_id TEXT NOT NULL,
    pv INTEGER NOT NULL DEFAULT 0 CHECK (pv >= 0),
    PRIMARY KEY (day, page_id)
);

CREATE INDEX idx_daily_page_stats_page_day
    ON daily_page_stats (page_id, day);

CREATE TABLE daily_referrers (
    day TEXT NOT NULL,
    page_id TEXT NOT NULL,
    source TEXT NOT NULL,
    pv INTEGER NOT NULL DEFAULT 0 CHECK (pv >= 0),
    PRIMARY KEY (day, page_id, source)
);

CREATE TABLE visitor_seen (
    day TEXT NOT NULL,
    visitor_hash BLOB NOT NULL CHECK (length(visitor_hash) = 32),
    PRIMARY KEY (day, visitor_hash)
);

CREATE TABLE event_seen (
    event_id TEXT PRIMARY KEY,
    expires_at INTEGER NOT NULL
);

CREATE INDEX idx_event_seen_expires
    ON event_seen (expires_at);
```

初始化时使用 `PRAGMA user_version` 标识 schema
版本。数据库结构变化必须有显式、可测试的迁移步骤；V1
初期只需支持从空数据库初始化。

不对 `daily_page_stats.page_id` 建立指向当前页面 Manifest
的外键，因为文章删除后历史统计仍需保留。

驱动采用 `database/sql` 配合纯 Go 的 [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)，具体依赖版本在 STA-01 验证后锁入 go.mod / go.sum。使用本地磁盘和 WAL；统计写入连接上限为 1，每个相关连接设置 5 秒 busy timeout，数据库操作受请求上下文和超时约束。以短事务读取报告，避免长期占用读快照。

报告使用只读数据库连接；部署允许 SQLite 管理所需的 WAL / SHM 辅助文件，并保证统计账户和报告账户有适当的目录及组权限。不能对正在变化的数据库使用 `immutable=1`。STA-01 与 STA-05 必须验证统计服务运行、停止和重启时报告均可读取；数据库只读意指不修改业务数据，辅助文件按 SQLite 规则管理。

### 13.2 事件事务

处理一个有效事件时，在同一 SQLite 事务中执行：

1.  插入 `event_seen`。若 `event_id`
    已存在，则不再更新任何计数，结束事务并返回已接受的重复事件结果。
2.  计算服务端当前站点日期与 `visitor_hash`。
3.  插入 `visitor_seen`；仅首次插入时增加当天全站 UV。
4.  增加当天全站 PV。
5.  增加当天对应页面 PV。
6.  增加当天该页面对应来源 PV。
7.  提交事务。

任一步失败都回滚整笔事务。使用短事务，避免在持有写锁时进行文件访问或其他慢操作。

### 13.3 数据清理与备份

-   `event_seen` 默认保留 72 小时，定期删除过期项。
-   `visitor_seen` 默认保留 7 天；每日 UV 汇总永久保留。
-   `daily_site_stats`、`daily_page_stats` 和 `daily_referrers` 不自动删除历史聚合数据。
-   统计 HMAC 密钥持久化保存，不能在每次重启时随机生成。
-   使用 SQLite 在线备份机制或 `VACUUM INTO`
    创建一致备份，不随意复制正在写入的单个 `.db` 文件。
-   必须测试从备份恢复后，数据库、密钥和报告任务可以继续运行。

## 14. HTTP API 契约

### 14.1 `POST /api/events`

这是 V1
唯一公开的动态写入端点。它不提供内容修改、文件写入、数据库查询或配置操作。

请求：

-   方法：`POST`
-   Content-Type：`application/json`
-   最大请求体：默认 2 KiB
-   不开放通用 CORS
-   Nginx 反向代理到 `127.0.0.1` 上的统计服务

请求 JSON：

``` json
{
  "v": 1,
  "event_id": "4c35b2fa7a0c4ad79a85ce5b9e720b43",
  "page_id": "post:nanoink-design",
  "visitor_id": "67e2a8d0c3b94f74a1d5e6f9082bc337",
  "referrer_host": "search.example"
}
```

字段规则：

  字段           必填   校验
  -------------- ------ --------------------------------------------------
  `v`            是     必须等于 `1`
  `event_id`     是     128 位随机数的小写十六进制形式，32 字符；单个页面事件稳定复用
  `page_id`      是     必须存在于当前 Manifest，为 Public 或 Unlisted 且启用统计
  `visitor_id`   是     128 位随机数的小写十六进制形式，32 字符
  `referrer_host` 是    主机名或 IP 字面值，最长 253；空字符串表示未知或直接访问

服务端按 12.3 区分来访来源和当前页面。请求体不得包含客户端日期、文件路径、完整 URL、访客 IP、User-Agent 或 SQL 字段；V1 严格拒绝未知字段。

响应：

  状态码                         含义
  ------------------------------ ----------------------------------------
  `204 No Content`               事件已接受，或属于已接受事件的重复提交
  `400 Bad Request`              JSON、字段或页面 ID 不合法
  `413 Content Too Large`        请求体超限
  `415 Unsupported Media Type`   Content-Type 不支持
  `429 Too Many Requests`        超出限流
  `503 Service Unavailable`      数据库或必要依赖暂时不可用

浏览器使用 `navigator.sendBeacon()`，以 `Blob` 指定
`application/json`。Beacon
适合页面卸载时发送小型事件，但不保证网络请求必然成功。统计漏计是可接受的，不能因此影响正文浏览。

ID 使用浏览器 `crypto.getRandomValues()` 生成。V1 在页面首次可见时发送一次事件；同一文档再次显示时不重复发送。`sendBeacon()` 返回成功仅表示入队，不代表服务器接收；入队失败时至多再试一次，复用事件 ID。新页面加载生成新事件；跨日标识以发送时的站点日期选择。

### 14.2 API 安全

-   Nginx 与 Go 双层限制请求大小和速率。
-   Nginx 按客户端连接地址限流；不经过 CDN
    时使用实际连接来源，不信任未经配置的转发头。
-   Go 验证 JSON 类型、字段长度、ID 格式和页面身份。
-   `Origin` / `Referer` 只能辅助筛查，不作为认证凭据。
-   仅接受预定方法和 JSON 类型；不开放任意跨域访问。
-   不向客户端返回数据库内部错误或文件路径。
-   统计服务不可用时，静态网站照常服务。

### 14.3 `GET /data/stats.json`

此 URL 对应 Nginx 提供的静态 JSON 文件，不是 Go 动态 API。

生成结构：

``` json
{
  "schema_version": 1,
  "build_id": "build-identifier",
  "generated_at": "2026-10-22T12:00:00+08:00",
  "pages": {
    "post:nanoink-design": {
      "views": 1284
    }
  },
  "popular_30d": [
    {
      "id": "post:nanoink-design",
      "title": "NanoInk 的硬件设计",
      "url": "/posts/nanoink-hardware/",
      "views": 384
    }
  ]
}
```

-   `pages.*.views` 是历史累计 PV。
-   `pages` 可包含全部启用统计的 Public HTML 页面；`popular_30d` 仅包含其中 kind 为 `post` 的文章，views 为最近 30 天 PV。
-   只输出本轮所用 Manifest 中的 Public 页面。
-   不输出 Unlisted 的 ID、标题、URL 或计数。
-   默认每 5 分钟刷新。
-   先写临时文件，再原子替换正式文件。
-   同一报告目标使用进程间排他锁避免并发覆盖。每轮基于单份 Manifest 和短数据库读事务生成两个输出；写入前发现当前 build_id 已变化则放弃并在下一轮重试。两个文件各自原子替换即可。
-   `build_id` 标明公开 JSON 使用的 Manifest 版本；Nginx 对该文件使用 `Cache-Control: no-cache`，客户端允许暂时显示旧结果。
-   客户端读取失败时可以隐藏统计数字或保留上次成功结果，不能阻塞页面阅读。

### 14.4 私有静态报告

`blog report` 或定时任务生成本地 `report.html`，包含每日趋势、7 天/30
天/累计数据、来源主机名、页面来源分布、热门页面以及 Unlisted 页面明细。页面按 Manifest 的标题和规范 URL 显示，历史已删除页面以 ID 显示。

报告不经公开 HTTP 路径提供。V1
使用本地文件访问；将来如需远程查看，再通过已有私有网络或额外认证入口单独设计，不在
V1 中实现登录系统。

## 15. 公开统计与内容版本一致性

-   静态页面和 Manifest 同属于一个发布版本。
-   统计服务发现当前版本指针变化时重新加载 Manifest。
-   Manifest 之外的未知页面 ID 拒绝统计。
-   Public 改为 Unlisted 后，仍可计入私有统计，在下一次成功刷新公开统计时移除该条目。
-   Unlisted 改为 Public 后，在新版本生效且下一次成功刷新报告后进入公开输出。
-   删除页面后历史聚合数据保留；该 ID 不再接受新的事件。
-   旧浏览器页面可能在发布切换后短时间继续上报；对已删除页面拒绝新事件是可接受的。
-   统计数据库、公开
    JSON、私有报告和博客发布目录相互分离。统计更新不触发站点重建，站点重建也不覆盖统计文件。

公开统计采用最终一致性：正常刷新周期为 5 分钟，报告失败时保留上次成功输出并记录错误，恢复后继续刷新；旧报告可能暂时显示先前的可见性与 URL。Unlisted 是公开可访问内容的列表选择，其可见性变更不要求报告与站点同步切换。

## 16. 安全原则

### 16.1 构建器

-   只处理配置允许的普通文件类型。
-   拒绝路径穿越、输出路径冲突和越界符号链接。
-   输入目录与输出目录分离。
-   不执行 Markdown 或 Front Matter 中的命令。
-   不允许内容元数据指定任意输出文件系统路径。
-   HTML/JS 的浏览器端自由度不意味着构建阶段可执行任意程序。

### 16.2 原生 HTML

文章和特殊 HTML 页面由作者自己编写，属于可信内容。系统允许自定义
DOM、CSS、JavaScript 和整页 HTML，不提供针对作者自身的脚本沙箱。

`html/template` 仍负责普通模板中的上下文转义。只有明确确认可信的
Markdown 正文 HTML 才可以绕过模板转义。不要把
`extra`、标题或其他普通元数据直接转换成可信 HTML。

由于自定义页面与站点同源，它们可以访问同源可见的数据和公开
API。不要声称它们被浏览器安全策略隔离。任何 CSP
配置都必须与作者的内联脚本和样式需求一起测试，不能以全局策略意外破坏特殊页面。

### 16.3 统计服务

-   仅接受最小事件 schema。
-   不读取源文件或 Git 凭据。
-   不接受文件路径或 SQL。
-   限制请求大小、频率和持久化增长。
-   不保存完整 IP、User-Agent 和完整 Referer。
-   SQLite、密钥与私有报告均不对公网开放。
-   把浏览器输入视为不可信，统计数据不作为安全或财务计量依据。

### 16.4 发布和恢复

-   构建先写入暂存目录。
-   通过所有校验后才发布。
-   原子切换当前版本。
-   失败时保留旧版本。
-   按保留数量保存完整 release，并验证人工回滚。
-   数据库使用受支持的备份方式，并定期验证恢复流程。

## 17. 测试不变量

以下行为必须有自动化测试覆盖：

1.  同一输入快照生成相同的内容索引、URL 集合和排序。
2.  修改 `updated` 不改变按 `published` 排序的结果。
3.  修改 slug 且保留显式 ID 时，URL 变化而统计身份不变。
4.  无显式摘要时产生稳定的纯文本摘要。
5.  Unlisted 可以通过 URL 访问，但不会进入公共列表、Feed、Sitemap；基于该版本 Manifest 新生成的公开统计不包含它。
6.  Draft 不出现在生产输出。
7.  一次构建只从经指纹校验的输入快照读取；后续输入变化能在下一次扫描发现。
8.  构建失败不改变当前版本。
9.  重复 `event_id` 不增加 PV 或 UV。
10. 同一天相同访客标识访问多页，只增加一次全站 UV。
11. 新的站点日期使用新的每日访客标识。
12. 数据库事务失败时不出现部分计数。
13. 非法页面 ID、超大请求和异常字段被拒绝。
14. 删除页面后历史统计保留，但该 ID 不再接受新事件。
15. 私有报告和 SQLite 不可通过公网 URL 读取。
16. 统计服务停止时公开站点继续工作。
17. 每个 HTML 页面都有唯一稳定 ID；分类总览、单分类与分页的 PV 分别累计。
18. 导航来源字段与当前页面身份分别记录，请求 Referer 不被误用为导航来源。
19. 无日期独立页面可以构建，其顺序在重复构建与重启后保持按稳定 ID 排列。
20. 展示名含 Unicode、大小写与 emoji 时不改变分类标签 ID；内容中的未配置标签忽略，文章分类按配置交集选择，分类配置中的未知父项和环失败。
21. 无尾斜杠与 index.html 访问重定向到同一个规范 URL，404 保持错误状态。
22. 静态构建无需统计配置；统计服务无需读取内容和模板；并发发布明确失败。
23. 公开报告按所用 Manifest 过滤，版本变化和任务失败遵守最终一致性规则。

## 18. 待实现时验证的事项

以下事项不应阻塞架构实现，但必须在相应任务完成时作出记录：

-   Goldmark 扩展组合是否完整满足现有文章语法；以真实示例内容测试。
-   数学公式使用 KaTeX 客户端渲染的具体资源打包方式。
-   代码高亮使用 Chroma 的语言覆盖与 CSS 样式。
-   HTML 独立页面统计脚本注入对格式的校验规则。
-   生产服务器的 Go、SQLite、systemd 和 Nginx 版本及配置细节；SQLite 驱动版本、WAL 辅助文件权限及锁等待验证。
-   以真实内容测量完整扫描和构建开销，确认已定义的分页大小、摘要长度和扫描间隔适合站点。
-   外部链接检查是否需要单独的手动命令；默认不阻断生产构建。
-   根据部署地区决定是否需要统计告知或同意机制。

任何验证结果导致已有契约需要变化时，先更新本文件和实施计划，再合并实现。

2026-10-23 本地验证：Go 1.27.0（Windows 与交叉编译的 Linux amd64）、modernc.org/sqlite 1.61.0 / SQLite 3.54.1、Nginx 1.30.6。Goldmark、Chroma、自托管 KaTeX 0.19.0、Standalone tokenizer、WAL/SHM 及取消锁等待均有自动测试；Edge 验证桌面/移动阅读与实际 Beacon 字段。18 页示例的一次完整扫描约 33 ms、完整构建约 318 ms，干净 Git 源码编译后的独立构建约 418 ms；这些是本机示例测量，不代表大型内容库性能。生产 Go/Nginx/systemd 环境、TLS 与部署地区的统计告知由实际部署方确认。

## 19. V1 完成标准

V1 完成时：

-   内容文件能够生成完整静态网站。
-   日期、身份、URL、可见性和排序行为符合本文。
-   RSS、Sitemap、SEO、阅读体验和特殊 HTML 页面可用。
-   定期扫描能发现新增、修改和删除内容。
-   构建失败不会破坏当前站点。
-   统计 API、SQLite 聚合、公开 JSON 和私有报告可用。
-   私有文件和统计数据库不经公网暴露。
-   `go test ./...`、`go vet ./...`、端到端构建测试和关键安全测试通过。
-   配置、部署、备份与恢复文档齐全。
