# 路径数据与节点

路径使用 `authoring.Path` 和 `authoring.PathTypeID`。它是可持久化的有序几何数据，
可作为节点输入/输出和工作流变量，不绑定游戏或定位插件。

路径含 version=1、reference、points。reference 包含 kind（world/local）、frame、unit、
axisHeading、axisSign、map、floor；未知 map/floor 用空字符串。每点含稳定 id、name、
x、y、z；未知高度 z=null，不使用零冒充未知。点数没有固定的产品限制，仍受宿主通用数据预算约束。

所有点共享坐标参照。观测 epoch 不属于持久路径身份。local 表示相对固定原点的局部坐标，
不是相对上一点的移动指令。必须显式指定目标坐标系、原点和 XY 平面逆时针旋转角进行对齐，
单位不能悄悄转换。坐标转换只改变几何，不证明定位正确或路段可通行。

正式 Catalog 已提供构建路径、获取路径点、截取路径、反转路径、拼接路径和对齐局部路径，均为纯数据节点。
序号从 0 开始，截取包含结束点。操作返回新值，不修改输入。拼接要求参照完全一致且点 ID 不重复；
重复点 ID 应由生成工具明确处理，不能悄悄改变被其他步骤引用的点位身份。

保存的资源使用 `authoring.PathAssetTypeID` 的 BlobRef，media type 为 `authoring.PathMediaType`
（application/vnd.yotta.path+json）。读取路径节点在显式 blob-read 边界读取固定内容，输出普通 Path；
纯数据节点不隐式读取外部资源。编辑素材库路径会产生新内容，已有工作流仍使用原 BlobRef。

宿主素材库提供路径录制编辑工具。定位插件声明 Frame.kind 和 Frame.recovery 后即可被工具消费，
无需实现自己的录制器。SDK 的旧 Frame 仍可解码，但路径工具不会猜测缺失的世界/局部参照。
`world`/`local` 描述坐标定义；`stable`/`align`/`session` 描述重连后的参照恢复方式。

沿路径移动要求持续更新的 position-source Snapshot JSON 字符串变量，以及 position、camera-heading。
路径点含 Z 时还要求新鲜 altitude；地图/楼层已声明时必须可用并匹配。与旧的 WorldPosition 变量不同，
此接口保留完整观测，而不改变已发布 WorldPosition TypeRef。每次 Run 验证路径一次，之后只读取当前点和新观测。
默认 start=0、end=-1（末点），每点有独立超时。输出 last-index（-1 表示尚未到达）、current-index、point-id、XY、distance。
timeout/stuck/unavailable/reference-mismatch/height-mismatch 提供独立出口；取消和所有退出均释放输入。
局部路径需先显式对齐为定位源的世界参照，不能直接把局部 XY 当成世界位置。

使用步骤见 [路径工具](../../../docs/product/paths.md)。

### 大路径资产

`navigation/follow-saved-path` 接收 `PathAssetTypeID` 的固定 BlobRef，声明 blob-read 能力，在开始执行时一次读取/校验，复用 `follow-path` 的逐点反馈控制和进度。它支持超过通用1MiB内联值容量的保存路径，文件预算仍为16MiB。`read-path` 超过内联预算返回 `path.inline_budget_exceeded` 并提示直接消费保存路径；不得扩大通用值预算或把资产字节冒充内联值。
