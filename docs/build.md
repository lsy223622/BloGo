# 构建与快照

输入由config.yaml、content/site.yaml、posts/pages、assets、templates及static组成。扫描以稳定相对路径、文件字节、程序摘要和预览选项计算SHA-256指纹；不依赖文件mtime。运行data和输出不进入输入扫描。

每次构建先创建私有快照，复核扫描指纹，再仅从快照解析和渲染。扫描后输入改变会使这次构建失败；快照完成后的编辑在下一轮发现。未知类型、特殊文件、符号链接和资源冲突明确失败。缺失图片、样式、脚本、Logo和favicon会失败；内容链接缺失默认警告，strict_links=true时失败。外部链接不下载、不发网络校验请求。

`blog build --output output/site`生成独立的public和meta。public是页面、静态资源、RSS、Sitemap和robots。meta包含Manifest、指纹和重定向数据。每份HTML都有对应页面身份。生产输出排除Draft，dev保留Draft但禁用其统计。

独立build先在同级临时目录生成完整版本，校验成功后替换；已有非空输出必须为可识别的BloGo构建。失败保留旧输出，成功不遗留已删除文章。

Linux serve/publish在staging完成构建及复核，移到releases后通过临时链接和原子rename切换current；相关目录必须在同一文件系统。serve在HTTP切换到新快照后清理旧版本，始终保留当前版本。路径与发布锁防止输入被输出覆盖及竞争发布。

Windows dev每次构建使用新的唯一目录，成功后切换HTTP快照，再清理旧目录，避免重命名正在提供页面的目录。浏览器修订号检查仅存在于dev；作者Standalone无法安全插入脚本时保持原文，手动刷新。

启用统计的页面要求自托管stats.js存在。全局关闭统计时不注入采集脚本、隐藏读数并禁用公开JSON/采集端点。普通页面请求只读取构建文件，不执行Markdown、模板或统计SQL。
