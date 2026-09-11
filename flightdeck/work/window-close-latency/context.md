# 上下文

产品实现归 Yotta，启动和进程管理使用 Workspace environments/event-runtime/run.ps1。使用 diagnosing-bugs 建立进程退出计时闭环；不把窗口隐藏当作退出，不以缩短超时替代正确清理。用户复测后已授权提交；未授权推送。

退出窗口的适配层不得在后台同步等待 UI 线程，因为 Wails 在该线程执行退出回调。普通关闭排队发出请求；Wails app context 取消后由框架统一销毁窗口。工具生命周期仍处理持有对象、未完成打开和采集清理，数据所有者仍完整关闭。

MCP 事件流是长期 HTTP 请求，http.Server.Shutdown 不会主动结束它。每个 MCP endpoint 的请求继承该 endpoint 的 context，关闭/禁用/替换时先 cancel，再等待 handler 结束；不能依赖客户端先断线。此 context 属于 HTTP endpoint，不是已提交工作流 Run 的生命周期。

UI 验收用资料副本制造未保存编辑，覆盖保存与放弃，并特意保持 MCP GET 连接，防止外部客户端是否恰好在线影响复现。最后恢复原资料启动修复版。

