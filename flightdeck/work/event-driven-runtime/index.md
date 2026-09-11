# 事件驱动运行时

## Goal

在唯一编译执行路径中支持周期事件、可挂起节点任务、监测打断与恢复，并使连续移动消费标准实时位置。

## Status

Finished — 运行时整改、无面板采集启动及两轴审查完成。
## Current

- 检查点：Yotta `482c2577`、Workspace `6df9a53`、位置插件 `079d530`。实现仍为未提交改动；没有 push 或部署。
- 唯一执行器已支持显式控制帧、Scope 任务树、异步 adapter/pull、周期与监测节点、嵌套打断恢复及活动时钟冻结。就绪队列/控制帧/worker 有界，长期运行不再按累计调用次数终止；逾期计时器之间让出就绪任务。
- 输入域协调覆盖跨别名、跨 Run 的桌面输入；暂停松键，处理结束后恢复持键意图。Application 使用16个有界 Run worker。调试任务树支持角色、状态、定位和折叠，沿用现有 UI。
- 标准 WorldPosition、构造/解析/拆分节点及 Move v2 已完成。位置通过独立 HTTP→解析→状态写入提供；远处持续 W、接近减速，恢复/转向后要求新样本，过期释放输入。模拟来源三次与 race 通过，真实游戏新路径已通过，详见下方实测结果。
- Node Contract v3 与正式 Wails 合同已生成。SDK `v0.0.0-local.h35563335a895640d` 已导出；位置插件1.2.0已打包并通过真实宿主导入执行。现有插件包保存 semantic contract，并非旧版外层合同，因此无需仅因 v3 强制重打全部插件；实际资料内1.1.1仍可加载。
- `task check` 最终通过：受影响 Go、合同、bindings、文档及154个前端文件/645项测试。完整 compiler/application/run/inputcoord race 与最后公平性定向 race 通过。真实 Windows 自动化、Process/Wasm 隔离和存储迁移 smoke 均通过。
- 最新 App 已构建、PE检查通过并用实际资料启动：`bin/event-runtime/Yotta.exe`，SHA256 `4994E148AE66927E710028770B87AD3AAF056EEF0FD0C1657AB3D590B49AD4B1`，PID8232。Workspace管理，CDP19418；实际用户作品当前修订23编译打开通过，读取前后源完全一致。
- 真实 WebView 已验主任务暂停/处理/恢复（约9.3秒），桌面及紧凑任务树审查通过。10ms周期100次日志全部成功，完整历史跨分段可读；高 CPU 负载下约2.5秒，非硬实时保证。
- 一次性工具已迁移实际资料573个 Run 和1个 terminal Move 工作流。23个作品身份、19,648条事件、2,013条值记录保留；只有“测试新节点”的源转换为6节点位置供给图，ID、revision19与目标 -259488/131920 保留并在新 App 编译打开。
- 备份位于实际资料 `backups/before-event-runtime-20260911T071052.006176400Z` 与 `backups/before-navigation-v2-20260911T071054.731172600Z`。未向实际资料加入合成测试作品。
- 用户进入场景后，真机发现 Move 持键会话漏声明 hold-button，违反底层完整操作合同。已改用 installed.HeldInputOperations()，新增同调用路径回归先红后绿；HTTP测试供给明确 /v1/position，日志断言改读完整 TimelineSnapshot。真机250单位目标2.28秒到达，节点距离3.43，停步后距离15.11（阈值20），连续三次坐标不变，无残留W。证据 .task/runtime-game-move-final.log 与 .task/event-runtime/move-after.json。最终修复已正式构建并由Workspace重启（PID25628）；原工作流在新App编译打开通过。race在重负载并行构建时触发过期停步；Go门禁通过，负载恢复后独立race通过21.688秒（.task/runtime-held-contract-race-isolated.log）。

## Next

None.

## Progress

2026-09-11：最后持键接口修复后，task check再次全部通过（含645项前端测试），独立race通过，异环真实移动及停步通过；新版App已由Workspace启动。未提交后续实现，未push或部署。

## References

- [使用与迁移说明](../../../docs/product/periodic-tasks.md)
- [Workspace 架构方案](../../../../workspace/flightdeck/work/event-driven-runtime/proposal.md)
- 验证日志：`.task/runtime-final-gate.log`、`.task/runtime-final-build.log`、`.task/runtime-plugin-isolation-final.log`、`.task/runtime-storage-final.log`、`.task/runtime-user-app-verified.log`。

## 本轮修复与审查

Run自动准备被引用的采集服务，使用同次配置快照并保留失败历史；面板不再是前置条件。真实克隆资料关闭面板/停止服务后，HTTP首跑及复用、再次冷启动Move到达均通过（`.task/event-runtime/companion-cold-result.json`）。首次坐标延迟回归先红后绿。

[两轴审查](references/review.md)共Standards四项、Spec两项，均已修复并定向/race复核。Windows完整原生输入smoke独立重跑通过（`.task/runtime-review-automation-isolated.log`）。并行Go门禁两项实时模拟移动触发过期停步，最终改串行。

最终验证：`GOFLAGS=-p=1 task check`通过（受影响Go、合同、bindings、文档、154文件/645前端测试）；冷启动移动独立race通过30.416秒。证据 `.task/runtime-companion-serial-check.log`、`.task/runtime-companion-cold-race-isolated.log`。最终App已启动并保持用户修订23不变。未提交、未push、未部署。
