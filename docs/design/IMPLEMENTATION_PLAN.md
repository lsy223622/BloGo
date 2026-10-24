# BloGo 工程实施索引

现行完整实施安排见[统一运行与首版发行实施计划](RELEASE_IMPLEMENTATION_PLAN.md)，行为契约见[技术设计](DESIGN.md)与[统一运行方案](RELEASE_SIMPLIFICATION.md)。

工程模块按以下依赖组织：

1. 严格配置、内容元数据、分类标签与资源输入。
2. 稳定身份、URL、索引、分页、RSS与Sitemap。
3. Markdown/HTML、默认主题及持久布局的局部导航。
4. SQLite汇总、详细事件事务、去重与私有报告。
5. 可信代理、HTTP路由、单进程serve/dev和调度。
6. 通用示例、白名单发行包、单服务部署和备份升级。
7. 实际包的Linux非root/Windows预览验收及独立审查。

验证覆盖错误输入保留、原子发布、并发事件去重、事务回滚、元数据转义、公开隐私、草稿排除、列表读数和无JS阅读。运行go test ./...、go vet ./...、Node浏览器模型测试与Python打包/导入测试；Linux权限和发布测试需原生Linux环境，不能以Windows交叉编译代替。

本地提交与发行包验收不等于远程发布。推送、版本标签、公开Release及生产部署均需用户后续明确指示。
