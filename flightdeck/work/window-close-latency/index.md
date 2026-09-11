# 窗口关闭延迟

## Status

Finished

## Goal

改善关闭窗口约 7 秒延迟，保留未保存编辑确认、保存、取消运行、释放输入和数据占用锁等正确清理。

## Current

- 两处原因均已修复；用户复测暂未发现问题，已授权提交。未推送。
- 扩展面板：Wails 在 UI 线程执行 OnShutdown，工具后台协程同步 Close 等待 UI，形成线程互等。窗口适配改为普通关闭异步投递；应用 context 取消后由 Wails 统一销毁窗口。相同面板场景从 5121ms 降到 109ms。
- 后续用户组合复测暴露第二处：MCP 的 GET 事件流保持连接，HTTP Shutdown 只停止新请求，未通知现有流结束，消耗完整 5 秒。线程栈确认 tools 已完成，停在 MCP Server.Shutdown；启动器只是并存窗口，不是此处根因。
- MCP 每个 endpoint 拥有请求 context；关闭、禁用或替换 endpoint 前取消请求，再等待 HTTP handler 退出；Prepare abort 也取消 context。未调小超时，未跳过 storage/runtime 清理。
- 新增真实 HTTP 长连接回归：客户端不主动断开时，应用关闭和禁用 endpoint 原先均超时；修复后正常结束。desktopapp/tools/appruntime/mcpserver race、task check、正式构建及实际候选 PE metadata 检查通过。
- 完整 GUI 验收使用资料副本：未保存工作流 + 扩展面板 + 快速启动器 + 特意保持的 MCP 流。放弃修改：确认框 51ms，点击后进程退出 485ms；保存退出：确认框 99ms，点击后进程退出 441ms。之前同组合为 5268ms。所有计时确认进程确实结束，无活动 Run。
- bin/shutdown-fix-2/Yotta.exe 已恢复正常用户资料启动，PID 25380。测试修改仅发生在 .task/event-runtime/profile 副本。

## Next

None

## References

- [上下文](context.md)

## 验证证据

.task/shutdown-combined-before.json、shutdown-stacks.txt、shutdown-mcp-red.log、shutdown-combined-race.log、shutdown-combined-check.log、shutdown-combined-build.log、shutdown-repro-combined-discard.json、shutdown-repro-combined-save.json。原生 GUI 复现脚本为 .task/shutdown-repro.cjs。临时线程栈探针已移除。计时不包含用户思考时间，不承诺正在执行任务时也固定半秒退出。

