# 实时位置源接口 v1

`yotta.position-source/v1` 将定位插件接入统一的数据边界。能力表示能提供的数据，不是授权项。
实现可以使用 OCR、小地图匹配或游戏适配器；消费者只依赖协议，不识别游戏名称。

## 创作者与工作流

1. 插件通过已配置的 HTTP target 提供 `GET /v1/position-source`（Descriptor）和
   `GET /v1/position-source/sample`（Snapshot）。响应为 200 JSON，禁止缓存。
2. 创作者声明支持的能力，逐项返回当前观测。消费者按自己的任务选择必需能力。
   记录途经点只要求 position；目前基于镜头转向的移动要求 position 和 camera-heading。
3. Go 插件使用本目录 `Decode` 校验；`Ready` 按消费者的新鲜度要求判断。
   HTTP GET → 解析实时位置 → 写入实时位置变量可供现有移动节点使用。
   旧的自定义 JSON 字段映射保持兼容。

这是一项数据协议和 SDK，不会自动新增插件商店标签、设置页或权限面板。

## 能力与值

| 能力 | value | 含义 |
| --- | --- | --- |
| position | `{ "x": 1, "y": 2 }` | 同一坐标框架中的平面位置 |
| altitude | number | 高度，使用 frame.unit |
| character-heading | number | 角色身体朝向，度数 [0,360) |
| camera-heading | number | 镜头朝向，度数 [0,360) |
| camera-pitch | number | 镜头俯仰，度数 [-90,90]，解码舍入容差 0.001 |
| map | string | 稳定地图标识，不能用展示名称冒充 |
| floor | string | 稳定楼层标识 |

Descriptor 含 protocol、source、capabilities、frame。Snapshot 额外含 epoch 和
observations（以 capability 为键）。声明的能力必须有 observation；不支持的能力不声明。
未知能力需升级协议，v1 消费者应明确拒绝，不能静默猜测。

每个 observation 包含 status、value、sampleTimeMs、sampleAgeMs、sequence、accuracy：

- tracking：有当前观测；消费者仍须按自己的时限检查。stale：保留了历史值。
- waiting：尚未得到观测。unavailable：采集失败或当前场景不支持；可保留历史值。
- 从未有值时 value=null、sampleTimeMs=0、sampleAgeMs=-1、sequence=0、accuracy=null。
- 有值时 Unix 毫秒时间为正，age 非负，sequence 为正且在 epoch 内按真实观测递增。
  静止时相同坐标也可以是新观测；轮询不能增加 sequence 或刷新采样时间。
- 每项能力独立保留采样时间。sampleAgeMs 用单调时钟计算；跨机器必须先转换到宿主时钟。
  消费者同时检查样本年龄和绝对时间，避免缓存或传输延迟让旧数据变新。
- accuracy 是已知的误差半径（坐标单位或角度）；未知填 null。不是猜测的置信度。

frame.id、unit 必须稳定，不能把像素、相对位移或未知原始单位说成米。
路径录制还要求 frame.kind（world/local）与 frame.recovery（stable/align/session）成对声明。
stable 表示重连后保持同一参照；align 表示须显式对齐；session 表示仅当前会话有效。
旧来源未声明这两项时仍兼容旧观测消费者，路径工具则明确要求更新来源，不根据游戏或插件名称推断。
axisHeading 是正 X 对应的朝向角；axisSign 为 +1/-1，说明 XY 角与朝向的方向关系。
角色和镜头朝向必须使用此同一参考方向。屏幕上的箭头角必须先扣除小地图旋转。
epoch 在进程重启、重新定位或坐标框架变化后更新。路线应绑定 source/frame/unit，
以及已知的 map/floor；未提供地图标识时不承诺跨地图路线匹配。
不同来源的观测只有在坐标系、朝向参照和时间兼容后才可组合；v1 不做自动校准或融合。

## NTE 适配与交付范围

NTE 声明 position、altitude、camera-heading、camera-pitch。坐标单位 nte-raw，
frame.id=nte/world，axisHeading=90、axisSign=1。没有角色朝向、地图和楼层能力，
精度未知。被动解码的有效标志表示格式与新鲜度可用，不保证任何游戏版本下绝对定位正确。
断线恢复时清除旧样本并更新 epoch；没有地图事件，因此不能检测所有切图。

原 NTE `/v1/position`、health 协议和读取节点的身份/端口不变。
新能力接口与旧接口从同一个加锁状态产生。v1.3.0 是插件包升级，旧节点合同仍是 v1.0.0。
NTE v1.4.0 进一步声明 world/stable；宿主路径工具使用同一采样接口录制、打点并执行点列。
它不提供障碍地图或自动绕障，也不把缺少的地图/楼层信息当作已匹配。
