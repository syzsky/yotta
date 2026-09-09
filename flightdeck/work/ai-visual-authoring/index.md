# AI 视觉创作与用户文档

## Status

Finished

## Goal

让外部 MCP 与内置 AI 提案读取当前创作上下文、默认目标和真实画面，辅助创建可审查工作流；建立面向用户的文档目录和可导入月离文档的 Release 附件。

## Current

文档归属已按 owner 后续纠正：最终正文、截图、脚本与 Actions 位于独立 yottaapp/docs 仓库；本仓的中间 user-docs/ 已迁出，产品 Release 不再重复打包 docs.zip。独立文档仓 content/ 保留全部 56 个包文件，包内容与迁移前完全一致。

已实现共用 authoringcontext，新增 authoring_context、automation_target、automation_capture；默认目标/指定目标与 Windows 虚拟桌面截图通过 MCP 和内置提案共用。图片经原生图像通道传给 Codex、OpenAI Responses 和 Anthropic。Codex 现在也传递 trusted manifest 的 developerInstructions；原适配器曾遗漏它。编辑器通过 Wails 同步工作流/图/dirty，提案固定在本轮工作流。

task check 通过：文档、Action 固定 SHA、合同、兼容性、离线 AI eval 8/8、Wails bindings、39 个受影响 Go 包及 629 前端测试。真实 Windows 屏幕截图通过，真实 Codex 图像工具识别红/蓝色块通过。真实 WebView/MCP 已验证活动工作流与截图，AI 侧栏能回复当前名称与画面信息。真实 Codex 从截图创建 text/concat 候选通过（15 轮、391668 输入 token、编译无错误、预览无副作用），未应用或运行。修正旧 25 万累计输入上限为每个允许轮次 64000 token，总费用/时间/调用限制不变，定向回归通过。泛型 Log 的模型端口猜测和类型修复失败另记 AI 提案验收 Work，不宣称任意节点生成已稳定。

独立文档仓库已完整接管正文、图片、打包脚本和 GitHub Actions；中文 26 篇、英文 23 篇及语言入口，docs.zip 共 56 文件。软件 Release 的重复文档打包已撤销，迁移后完整包 SHA256 不变。

本地 App 已使用 bin/visual-authoring-final/Yotta.exe，Workspace Session 20260909T150025Z-42920，默认 bin/Yotta.exe 已同步，继续连接线上服务。保留上一轮校准修复，未提交、push 或上传文档。最终真实侧栏验收退出 0：识别工作流名称及“运行”按钮，再截图并将摘要写入 text/concat，候选编译通过、预览无变更。临时提案拒绝、工作流与对话删除，AI/MCP 配置恢复。此前 Playwright 留图/点击因窗口状态超时，恢复窗口后实际交互通过；留图改用已验证的原生截图接口。

## Next

None

## References

- [稳定上下文](context.md)
- [截图指引](../../knowledge/automation/input-and-capture.md)
- [错误契约](../../knowledge/errors/error-contract.md)
