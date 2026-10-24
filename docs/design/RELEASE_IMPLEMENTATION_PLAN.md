# 统一运行与首版发行实施计划

> **For agentic workers:** 使用 superpowers:executing-plans 在现有开发会话按任务执行；用户已指定该会话及 Sol/high，阶段结束继续下一项。独立审查遵守用户的模型成本限制。

**Goal:** 交付解压后以单进程、单端口运行的完整博客包，包含易编辑配置、附件、可选详细统计、列表阅读量和 GitHub Actions 发行流程。

**Architecture:** 复用 Go 静态构建与 Linux 原子发布。统一 HTTP 服务在 127.0.0.1:25646 提供公开站点和统计，进程内调度扫描、报告和清理；用户输入统一存于 content，运行数据存于私有 data。

**Tech Stack:** Go 1.27+、现有 Goldmark/Chroma/SQLite、原生 HTTP 与现有主题 JS、GitHub Actions；运行包不依赖 Go/Node/Python。

**Spec:** [RELEASE_SIMPLIFICATION.md](RELEASE_SIMPLIFICATION.md)，2026-10-25 用户确认版本。基线 169c875，随后仅有本次方案定稿提交。

状态：全部8项已完成本地实现与实际包验收。独立审查后的修复提交为 `11910bc`。Linux amd64完成普通用户容器验收，arm64包在架构兼容环境运行；Windows包完成dev/build与真实浏览器验收。云端工作流需后续远端触发。

## 全局约束

- 完成全部任务后交付，不把某项完成或阶段总结当作任务终点。
- 默认端口 25646；一个运行用户、一份后台配置、一个可选 systemd service，无外部报告 timer。
- config.yaml 放后台设置；content/site.yaml 合并展示与分类标签；content/assets 映射 /media/。
- 正式 serve 面向 Linux amd64/arm64；Windows amd64 包提供 dev/build，并能本地试用非草稿统计。
- 默认详细统计关闭，明细保留 168h，可配置；公开输出不包含个人明细。
- 不恢复旧资源宽限继承，不增加后台 CMS、自动自更新、通用兼容框架，不做性能对照测试。
- 使用当前源码、常规路径及职责；以下接口是跨任务接点，内部模型可按已有代码合理整理，不制造无用途抽象。
- 原博文仓库、原 SVG 和 Typecho 备份只读；个人输入、统计、密钥、临时工具不提交或进入发行包。
- 允许本地代码提交；不自动推送、创建公开 Release、打远程标签或部署生产。

## Review Focus

1. 首次启动空 data 和重启已有 data：创建正确，保留原密钥及历史，第二实例拒绝；任务 3/5/8 验证。
2. 转发头来自陌生对端、伪造左侧链、IPv6 和非法地址：不能冒充客户端 IP 绕过限流；任务 2 验证。
3. 内容/展示或运行配置写坏、报告失败、构建时变更：不替换有效版本，不混用身份/时区，不无限堆叠任务；任务 1/5 验证。
4. 明细开关、重复事件、事务中断、保留期缩短：不部分计数、不新增重复流水、不删除汇总；任务 3 验证。
5. Release 全量解压及升级：个人内容、密钥不被默认文件覆盖；HTTP 不暴露私有文件；任务 4/7/8 验证。

## 任务 1：配置、内容设置与附件输入

**Files:** 修改 internal/config/config.go、paths.go 及测试；新增 internal/config/content.go 与测试；修改 internal/content/taxonomies.go、internal/site/load.go、internal/build/snapshot.go、assets.go、build.go 和相应测试、样例配置。

**Interfaces:** 保留 config.Load(file string) (*Config, error) 为入口，新增 config.LoadContentSettings(file string) (*ContentSettings, error)。ContentSettings 定义 schema_version、title/subtitle/language、page_size、summary_max_chars、logo/favicon、profile、categories/tags；有效内部 Config 继续向现有构建/模板提供站点及分类标签数据，避免多次独立解析同一份 YAML。站点根目录为主配置目录，data/content/templates/static 默认相对该根。

- [x] 写失败测试 TestLoadRuntimeAndContentSettings：最小配置得到 127.0.0.1:25646、60s、5m、details=false、168h；显示字段只从 content/site.yaml 读取，重复和未知键失败，中文展示名保留，分类父环失败，跨工作目录显式 --config 路径仍正确。
- [x] 写失败测试 TestContentSettingsAndAssetsSnapshot：仅修改 site.yaml 或附件字节即改变指纹；assets/photo.png 输出 /media/photo.png；缺失资源、静态冲突、输入中的符号链接失败；输出路径和数据目录不能覆盖输入。
- [x] 运行 go test ./internal/config ./internal/content ./internal/build ./internal/site 确认新契约测试失败；实现新外部配置、默认路径和统一快照读取。content 下只将 posts/pages 当页面，site.yaml 和 assets 分别处理，不能被旧 readTree 白名单误拒绝或重复计入。
- [x] 运行相同测试通过，更新构建/内容文档及配置样例，提交。

## 任务 2：可信反代地址与按来源限流

**Files:** 新增 internal/server/clientip.go、clientip_test.go；修改 internal/stats/api.go、api_test.go。

**Interfaces:** server.ClientIP(r *http.Request, trusted []netip.Prefix) (netip.Addr, error)，在统一 HTTP 入口处理后传递可信解析结果给采集器；解析与限流不能信任 JSON 自报 IP。handler 请求上下文保存结果，避免 stats 反向导入 server 形成循环；轻量共用类型可放 internal/requestmeta。

- [x] 写 TestClientIPTrustedChain：陌生对端忽略 XFF；可信回环代理链取右侧最近非可信节点，伪造最左值无效；多值头合并、IPv6、空值和非法值按设计处理。
- [x] 写 TestCollectorLimitsByResolvedIP：同一反代后的两个真实 IP 分开限流；陌生请求伪造头仍共享其真实来源桶；空闲来源状态有回收，不能无界增长。
- [x] 运行 go test ./internal/server ./internal/stats 确认失败，再实现地址规范化和有界的来源限流，保留既有请求总量限制及 2KiB、严格字段校验。
- [x] 测试通过后提交，补反代示例的 XFF 转发规则。

## 任务 3：详细事件存储、迁移与报告

**Files:** 修改 internal/stats/store.go、api.go、report.go、server.go 及测试；按职责新增 details.go、details_test.go，保留 key.go 的持久密钥逻辑。

**Interfaces:** 扩展 stats.Record 及配置，保持 Store.Record(ctx, record) (bool, error)、Cleanup 和 GenerateReport 的核心入口。新增具体详细事件结构记录时间、事件和页面身份、当时 URL/标题、客户端 IP、UA、语言、当日访客摘要。报告从 UA 得到粗粒度浏览器/系统/设备；选用小型维护中的 Go 方案或现有可靠数据解析，未知不猜。

- [x] 写 TestUpgradeV1PreservesAggregates：版本 1 数据库按事务升级到新 schema（建议 user_version=2），原 PV/UV、来源、去重记录保留，重复启动不重建；迁移失败回滚，未知版本仍明确失败。
- [x] 写 TestDetailsTransactionalToggle：开启时有效事件只记一条；重复不增 PV/明细；事务故障全部回滚；关闭时无新明细但汇总正常；重新开启不补造旧事件。
- [x] 写 TestDetailsRetentionAndReportPrivacy：168h 过期及保留期缩短清理现存明细，汇总不删；最多最近1000条进入私有 HTML 并注明保留总数；公开 JSON 无 IP、UA、语言、访客摘要、Unlisted 明细。IP 使用任务2结果；UA和语言做长度限制及 HTML 转义。
- [x] 用真实 UA 检查桌面/手机/未知分类和首选语言；不新增客户端探测字段，不改变原五字段事件协议和每日 UV 口径。
- [x] 运行 go test ./internal/stats 先失败后通过，报告/备份恢复测试通过，提交并更新统计契约。

## 任务 4：正式 HTTP 路由

**Files:** 新增 internal/server/http.go、http_test.go；复用或抽取 internal/preview/server.go 的规范路由功能，修改对应测试。

**Interfaces:** server.NewHandler(c *config.Config, published *build.Result, collector http.Handler) (http.Handler, error)。只接当前 public 输出、采集器和指定公开 JSON；内部替换发布结果的机制由实现者选择并保持线程安全。

- [x] 写表驱动 HTTP 测试：GET/HEAD、正确 Content-Type、首页文章资源、404 保持404、无尾斜杠和 index.html 的301保留query、aliases随发布更新、非GET/HEAD仅API允许POST、哈希资源 immutable、HTML/统计 no-cache。
- [x] 写 TestPrivatePathsNotServed：config.yaml、content/site.yaml、data、Manifest、密钥、数据库、私有报告、穿越和目录列表均不能经 HTTP 获取；/data/stats.json 只映射单文件。
- [x] 用 httptest 将真实 stats.NewHandler 接入 /api/events，验证有效事件204和来源/页面校验，URL不能通过泛文件服务旁路；stats.enabled=false 不接收事件也不暴露旧 JSON。
- [x] 运行 go test ./internal/server ./internal/preview ./internal/stats，先确认失败再实现，测试通过后提交。

## 任务 5：serve 与进程内调度、本地 dev

**Files:** 新增 internal/server/serve.go、serve_test.go；修改 cmd/blog/main.go 与测试，复用 internal/build/watch.go、publish.go；调整 internal/preview/server.go、internal/stats/server.go 的可复用生命周期。

**Interfaces:** server.Serve(ctx context.Context, c *config.Config, log io.Writer) error，为 CLI 新增 serve。所有任务在当前进程协调，不能 shell 启动 watch/stats/report。stats Serve 拆出可复用采集器初始化/关闭，现有命令可继续用于手工调试；默认文档只围绕新正常流程。

- [x] 写 TestServeFirstRunAndRestart：空 data 自动建立发布/统计/密钥，重启延续数据；第二实例同 data 拒绝；首次无可用版本构建错误明确退出，后续错误保留旧版本。
- [x] 写 TestServeReloadAndSchedule：运行中缩短扫描/报告间隔确实改变执行周期；报告超时/失败不叠加、保留旧结果；详细开关和保留期动态生效；监听、base_url、timezone变化只提示需重启，不能应用半份配置。坏 YAML 保留最后有效运行设置。
- [x] 写 TestDevStatsExcludeDrafts：dev 草稿可见但无采集；公有/不列出/系统页可本机统计，使用 data/dev 独立数据，正式 Manifest 不含草稿。dev 从明确的本机监听地址派生预览 origin 用于链接及采集校验，不要求用户来回改正式 base_url，也不从任意 Host 头推导可信 origin。复用任务4路由，不保留临时 Node 接线。
- [x] 验证 Ctrl+C/SIGTERM、锁释放、HTTP超时/连接关闭、数据库关闭和持久密钥。serve 不依赖 Nginx、systemd 或外部运行时；只需一个普通用户能写 data。
- [x] 运行 go test ./...、go vet ./... 及 Windows dev 与 Linux serve 集成后提交。

## 任务 6：品牌路径与列表阅读量

**Files:** 修改 internal/render/render.go、render_test.go、templates/page.html、static/assets/theme.js、theme.css、scripts/theme-browser.test.cjs。

**Interfaces:** 使用任务1 ContentSettings 的 Logo/favicon/profile；列表数据节点以 ContentItem.ID 填 data-views-for，沿用 PublicReport.pages[id].views 累计口径，不新增逐条 API。

- [x] 写 TestListViewCountNodes：首页、分类、标签、归档、更新和分页的文章项均输出正确稳定ID；正文相同。有有效0显示0，全局或单页关闭、缺失、失败时隐藏，不将近30天排名数字误作累计数；全局关闭不注入采集脚本，草稿不触发采集。
- [x] 写浏览器回归：一份JSON填充多个节点；Swup切换到列表后能填充且不重复；统计请求失败不阻断阅读；无JS保留文章及导航。
- [x] 写品牌路径测试：通用默认图标和 content/assets 个人图标均有正确 URL、比例、浅深色及 favicon；对元数据继续上下文转义。
- [x] 运行 go test ./internal/render 和 Node统计/主题测试，真实桌面/手机检查后提交。

## 任务 7：开箱输入、部署与发行工作流

**Files:** 修改 config.yaml、content/、static/images/logo.svg 和 favicon.svg、README.md、docs/、deploy/；新增 content/site.yaml、content/assets/ 示例、.github/workflows/ci.yml、release.yml、scripts/package-release.* 和打包清单测试。脚本语言按现有环境选择，不引入发行框架。

**Interfaces:** 打包脚本输入版本和已编译目标文件，输出 linux-amd64/linux-arm64 tar.gz、windows-amd64预览zip、SHA256SUMS；只从白名单收集运行资源和许可。CI验证通过后，v*标签工作流构建并解压验收，再允许release任务上传；默认jobs contents:read，发布job才contents:write。

- [x] 先写打包清单断言：含程序、两份配置、模板、静态依赖及许可、示例文章、说明和单服务示例；不含个人内容、备份、原Logo、.git、output、运行data、密钥或开发依赖。
- [x] 默认示例可启动，包含快速开始、全部元数据与精简模板、状态/分类、附件、Markdown/TOC/代码/公式/Mermaid、配置和升级说明；通用默认图标不复用用户个人网站图形。删除过时多用户/多service/timer默认部署材料及引用，不加宣布删除的占位文案。
- [x] 单 service 只管理生命周期，反代示例只转发统一端口并传正确头；site.yaml 提供可直接编辑的注释与推荐值。默认详细模式false、保留168h、报告5m、端口25646。
- [x] 实现升级验收：新包解压到新目录，迁移 config/content/data 后保留文章ID、附件摘要、配置、原密钥和统计；默认主题跟新包，自定义模板按说明合并。README中正常用户不需要编译程序。
- [x] 本地构建三目标并检查归档内容、校验和和解压运行；新建workflow是待后续推送后运行的配置，不宣称GitHub云端CI已经成功，也不在本任务主动推送或发布。
- [x] 同步 DESIGN.md、既有 IMPLEMENTATION_PLAN.md 与专题文档，使现行契约一致，提交。

## 任务 8：个人数据迁移、完整包验收与交付

**Files:** 修改 internal/integration/ 测试与 scripts/verify-stack.ps1；个人内容迁移结果及截图仅放 output/；按需更新原导入工具适应新目录，不重跑会覆盖真实元数据的旧试用导入。

**Interfaces:** 使用实际解压包及任务5 serve/dev。复用原 Linux 原子发布/统计备份测试，并将多账户部署夹具改为单普通用户；公网反代用隔离测试实例。

- [x] 备份当前 output/personal-preview 输入及统计；迁移19份内容、合并site与分类标签，保留12 Public/2 Unlisted/4 Draft和友链语义、已整理元数据/文章ID/aliases，正文除明确的本地资源引用替换外不变。15个已发布内容已开启统计，草稿关闭。6张 Typecho 图片迁到content/assets/typecho，精确改正文/HTML/封面引用并核对摘要；个人Logo移入个人content/assets，不进默认仓库。
- [x] 从包启动，真实浏览器走反代访问→正确客户端IP→事件→数据库→按配置生成报告→文章及列表读数；验证单次可见、Swup、重复去重、明细开关、重启延续。私有报告核对IP/UA/语言/来源/页面及分类显示；默认关闭时没有明细新增。
- [x] 无构建工具的 Linux 解压环境验证首次启动、热更新、错误保留、301/404、禁私有访问、停止重启、恢复、单用户权限；Windows包验证dev/build、本地统计、热重建。当前旧Windows预览有rename Access denied，必须在新输出用当前包核对，不能用冻结旧输出宣称通过。
- [x] 运行 go test ./...、go vet ./...、Node统计/主题测试及仍适用的Python导入测试；完成与本轮变化相关的Linux权限/发布/备份集成。不要进行用户已取消的性能对照测试。
- [x] 做一次完整改动的独立只读审查，修复确认的重要问题。收尾检查源码diff、个人文件泄漏和归档清单，保存证据。
- [x] 最终交付提交、所有任务状态、实际测试与限制、可用本机试用和私有报告、发行包/校验文件位置及用户版部署升级流程。无事实依据不能宣称云端Actions或生产部署已通过。
