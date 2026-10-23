# 本地博文试用

使用 Python 3.11+ 与 Git，将 Markdown 工作区导入仓库已忽略的 `output/`，不修改原仓库：

```powershell
python scripts/import-preview.py --source 'C:/path/to/Blogs' --older 'C:/path/to/older/Blog' --attachments 'C:/path/to/pics'
go run ./cmd/blog dev --config output/personal-preview/config.yaml --output site --listen 127.0.0.1:8771
```

`--older` 和 `--attachments` 可省略；`--output` 默认为 `output/personal-preview`。导入使用当前文件，包括未提交修改。较旧副本只比较相同文件、忽略大小写及文件名中的更新日期差异；不同版本保留主要来源，未匹配的旧文件列入说明供核对，不静默合并。

结果包含独立 content、static、taxonomies.yaml、config.yaml、逐篇 `import-map.json` 和 `IMPORT.md`。ID 根据源文件名确定，正文修改不会改变 ID。映射记录源文件与正文摘要、输出 URL、日期依据和转换记录；试用输入、个人文章与构建结果都不提交为开源示例。

正文明确写明首次发布日期时采用该日期；其他文章的 Git 首次记录仅作试用近似值，无法查到时留空，不使用文件 mtime 或发行版本表格日期。不确定公开性的文章为 Draft，只有 dev 可见。友链为无日期独立页面，空“关于”跳过。分类标签是本地整理，不宣称为旧站原始元数据。

旧 `[TOC]` 标记保留，相邻空目录标题在代码围栏之外移除，由渲染器根据标记生成真实目录。两条已知旧分类链接映射到本地分类；其他正文、原生 HTML 和外链图片保留。附件只盘点，不下载远程图片；有未解析本地资源时构建会按正常资源检查失败，需核对原路径后再处理。

导入前检查源目录、Markdown 清单和 UTF-8，错误输入保留上次试用数据。重复导入只更新映射记录中的生成文章，不覆盖不认识的目录。修改项目主题资源后重新执行导入，将 static 副本刷新；模板直接引用项目模板，dev 会检测变化。生成配置只提供回环预览所需路径。

```powershell
python -m unittest discover -s scripts -p test_import_preview.py
```
