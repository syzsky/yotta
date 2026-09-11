# 面板与运行信号

## Status

Finished

## Goal

按钮通过事件队列唤醒工作流，支持持续监听、下一步等待和 Run 内信号；多个 Run 显式订阅同一面板时独立广播。

## Current

- 已交付“监听面板信号”“发送信号”“监听信号”“等待信号”。面板广播给全部显式订阅者；普通命名信号只在当前 Run 内通信。监听分支串行执行，主任务结束、停止或取消会释放订阅，面板保留。
- 持续监听最多排队 64 个事件、1 MiB 数据，超量明确报错；暂停时保留事件、恢复后处理。单次等待只保留订阅后的首个事件，不重放旧点击；超时使用活动时间。连续按钮请求依次提交，一次等待按钮提交期间禁用。
- Invoke.Subscription 由执行器管理续接，adapter 只提供事件源；Node Contract v3 是本次整改尚未提交/发布的生成物，已正式重新生成，已提交 v1/v2 未修改。
- 完整 task check 通过：Go、合同、生成绑定、格式、类型、i18n 以及前端 154 文件 / 646 测试。定向 race 覆盖共享广播、Run 隔离、停止、暂停及等待超时，全部通过。
- 真实生产 Wails 浮窗验收通过：同一按钮连续点击 3 次，两个 Run 各计数 3；两次 Next 各推进一阶段；两个 Run 成功结束、订阅释放、按钮随界面刷新禁用。验收仅操作隔离资料。
- bin/panel-signals/Yotta.exe 构建及实际路径 PE 检查通过。SHA256 8769BA4C67B8DC5BEA0D0BFC54808AB6E275592FFF86EFB4D98F4DD891317562。Workspace 已恢复原资料并启动该新版 PID 31388；25 个工作流修订和 Source 哈希逐一保持不变。未提交、推送或部署。

## Next

None

## References

- [上下文](context.md)
- [使用说明](../../../docs/product/periodic-tasks.md)
- 验收证据：.task/panel-signals/acceptance.json、acceptance.png、user-before.json、user-after.json；完整检查 .task/panel-signals-final-check.log；race .task/panel-signals-race-final.log。
