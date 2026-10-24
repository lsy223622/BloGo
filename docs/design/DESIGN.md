# BloGo 技术设计

当前契约以本文件和[统一运行方案](RELEASE_SIMPLIFICATION.md)为准，实施安排见[发行计划](RELEASE_IMPLEMENTATION_PLAN.md)。

## 架构

单作者、本地文件驱动的Go静态博客。内容权威源是Markdown/HTML，Go预先生成完整多页面HTML。一个serve进程负责HTTP、统计事件、输入扫描、报告和清理；请求页面不执行模板、Markdown或SQL。反向代理只处理域名/TLS并转发统一端口25646。浏览器用原生JavaScript及Swup渐进增强，关闭JavaScript仍可阅读。

Linux amd64/arm64为正式发布目标，Windows amd64提供dev/build。Linux原子发布需本地同一文件系统；运行不依赖Go、Node、Python、数据库服务器或额外调度器。

## 输入与配置

config.yaml是严格运行配置，定义server、timezone、paths、build、stats；未知键、重复键、类型错误失败。相对路径相对于配置目录。content/site.yaml保存schema_version、title/subtitle/language、page_size、summary_max_chars、Logo/favicon、profile以及categories/tags。

content/posts与pages保存带YAML Front Matter的Markdown/HTML。content/assets映射/media；static映射网站根目录，/assets保留给受管资源。路径冲突、符号链接、特殊文件、缺失本地资源失败。运行data不能覆盖或包含输入。个人输入只存在用户内容目录，不随默认主题升级覆盖。

## 内容身份与公开性

14个基础元数据字段为id、slug、title、status、published、updated、layout、permalink、aliases、categories、tags、summary、stats、extra。id缺省由slug推导，页面身份带post:/page:前缀；显式id在URL变化后维持统计身份。日期使用IANA时区，Public/Unlisted文章需要发布日期，独立页面可无日期，排序不使用mtime。

Public进入公共列表、RSS和Sitemap；Unlisted可直接访问、带noindex且不进入公共聚合；Draft只在dev生成，不采集统计。分类/标签仅保留配置与文章引用的交集；未配置分类不阻止生成。公共分类导航、标签列表只取Public文章，分类包含祖先且按文章去重。

分页、归档和系统页面使用稳定身份。规范目录URL、index.html和aliases由Go返回301并保留query。未知地址及404页面保持404，不提供目录列表。详情见[内容契约](../content-output.md)和[格式示例](../content-example.md)。

## 构建、发布与服务

所有输入字节、路径、程序摘要及预览模式参与指纹。构建创建完整只读语义快照并复核指纹，解析、索引、渲染和资源检查均从快照读取。Linux先构建staging，再移动完整release并原子切换current；HTTP使用线程安全快照，切换后清理旧版本。失败保留上次成功输出。

serve启动取得data/run.lock，拒绝同数据的第二实例；创建私有运行目录、校验数据库与原访客密钥，初始化发布及统计报告。数据库存在而密钥缺失时失败。运行中串行扫描、报告和清理；每个统计任务有10秒期限，失败保留旧结果并重试。SIGINT/SIGTERM停止调度，关闭HTTP与数据库、释放锁。

运行配置独立检查；扫描/报告间隔、统计及明细开关、保留期可更新；监听、origin、时区及路径变化整份拒绝热应用并提示重启。错误YAML保留有效设置。dev从明确回环监听地址推导origin，使用data/dev和过滤草稿后的采集Manifest，每次重建使用唯一目录。

公网页面处理器只提供当前release/public、/api/events与/data/stats.json。配置、源文件、meta、数据库、密钥和私有报告均不可经HTTP读取。HTML/JSON no-cache，哈希CSS/JS immutable；GET/HEAD有效，事件端点使用POST和严格五字段协议。

## 统计与隐私

事件包含v/event_id/page_id/visitor_id/referrer_host。请求限制2KiB、严格JSON、页面清单及origin校验、总量及有界按来源限流。可信代理IP/CIDR由运行配置定义；只有直接连接可信代理时才从右向左解析XFF，取最近的不可信地址，IPv4映射地址规范化。UV继续使用按站点日期HMAC的随机访客标识，不以IP去重。

SQLite单写者、WAL、事务更新每日全站PV/UV、逐页PV、来源和去重记录。schema1到2迁移原子完成并保留历史。详情默认关闭；开启时同一计数事务增加服务端时间、事件/页面身份、当时规范URL/标题、解析IP、来源主机名、有界UA、首选语言、每日访客摘要及粗粒度浏览器/系统/设备。重复事件不增加计数或明细；明细到期不删汇总。

默认事件去重72h、访客摘要168h、明细168h，保留期可缩短。详细记录存在时仍保护相同事件不重复计数。私有HTML最多展示最近1000条并标明保留总数。公开JSON只含当前Manifest允许公开的计数和热门文章，不暴露Unlisted身份或个人字段。HTTP会按当前发布再次过滤公开JSON。

正文及列表复用一次公开JSON，以稳定ID填累计阅读量；有效0显示0，关闭/缺失/失败隐藏。局部导航后重新填充，不逐项请求API。备份需同时保存原密钥、数据库与内容，详见[统计契约](../statistics.md)。

## 默认主题与发行

主题提供三种主题模式、层级目录、代码换行/复制、公式/Mermaid、灯箱、移动端固定导航和持久公共布局。Swup只替换main，导航、分栏线和信息栏保持DOM；脚本按页面生命周期初始化，尊重减少动态效果。作者脚本页面和异常导航回到完整加载。

发行包按白名单收集程序、默认config/content/templates/static/deploy/docs和许可证；个人内容、运行data、备份、源Logo及开发依赖不进入包。Actions测试后从实际压缩包解压验收；只有发布job有contents:write。升级在新目录迁移config/content/data，不覆盖用户输入。没有自动更新、后台CMS或多实例统计写入机制。
