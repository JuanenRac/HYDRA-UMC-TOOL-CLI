<p align="center">
  <img src="images/HYDRA_UMC_BANNER.svg" alt="HYDRA-UMC-TOOL-CLI banner" width="100%">
</p>

# 💻 HYDRA-UMC-TOOL-CLI

<p align="center"><a href="README.md">🇺🇸 English</a> | <a href="README_spa.md">🇪🇸 Español</a> | <a href="README_fra.md">🇫🇷 Français</a> | <a href="README_ita.md">🇮🇹 Italiano</a> | <a href="README_deu.md">🇩🇪 Deutsch</a> | 🇨🇳 <b>简体中文</b> | <a href="README_jpn.md">🇯🇵 日本語</a></p>

### 🛠️ 面向车队 DevOps 与自动化的命令行界面

<p align="left">
  <img src="https://img.shields.io/badge/Licencia-GPL%203.0-blue.svg" alt="GPL 3.0">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Feature-Fleet%20DevOps-blue.svg" alt="DevOps">
</p>

---

## 1. 🛠️ 技术概述

**HYDRA-UMC-TOOL-CLI** 是 HYDRA-UMC 生态系统开发者和系统管理员的瑞士
军刀。它是一个单一的静态 Go 二进制文件，提供用于查询、更新和审计
HYDRA-UMC 部署的命令行工具。

长期目标是直接从终端或 CI/CD 流水线实现大规模任务部署、并行固件更新
（CAN-OTA）以及深度系统诊断。今天它已交付了其余一切功能所依赖的真实
可用基础：版本报告以及针对 HYDRA-UMC-SERVER 的实时 HTTP 客户端。

### 关键特性：
* ✅ **`hydra-cli version`** —— 打印 CLI 自身的名称和版本。*（已实现）*
* ✅ **`hydra-cli status [--server URL]`** —— 查询实时 HYDRA-UMC-SERVER 的 `GET /api/hydra-info`，并打印其报告的身份信息。*（已实现）*
* ✅ **`hydra-cli robots [--server URL]`** —— 查询实时 HYDRA-UMC-SERVER 的 `GET /api/settings`，并打印其真实的控制器/机器人名单（名称、在线状态、型号、角色）。*（已实现）*
* ✅ **`hydra-cli doctor [--server URL]`** —— 只读服务器契约诊断：验证 `/api/hydra-info` 和 `/api/settings`，并将已发布的控制器/机器人总数与名单进行核对。不发送命令，也不探测硬件。*（已实现）*
* ✅ **真实、稳定的退出码契约** —— `0` 正常，`1` 一般错误，`2` 用法错误，`3` 配置错误，`4` 网络错误，`5` 服务器错误，`6` 未实现。每个命令都通过该契约对自身的失败进行分类，而不是简单地 `exit 1`，因此封装本 CLI 的脚本可以根据*失败原因*进行分支处理。*（已实现）*
* ✅ **`hydra-cli config validate --config PATH`** —— 加载并按模式校验本地配置文件（服务器 URL、请求超时）。*（已实现）*
* ✅ **`hydra-cli config apply --config PATH [--dry-run]`** —— `--dry-run` 端到端地验证真实的校验路径，并准确打印将要发送的内容；若不带该参数，则会如实返回“未实现”，因为目前尚不存在实时的车队写入端点。*（已实现，仅支持 dry-run）*
* ✅ **`hydra-cli shell [--server URL]`** —— 一个交互式 REPL：无需重启进程即可对同一服务器重复运行上面的任意命令。通过与单次调用完全相同的命令表进行分发，因此 shell 与单次调用的行为永远不会出现分歧。输入 `exit`/`quit` 或按 Ctrl-D 退出。*（已实现）*
* ✅ **`hydra-cli help` / `--help`** —— 完整的命令用法。*（已实现）*
* 🚧 **`hydra-cli deploy`** —— 同时向一批机器人上传任务和配置。*（计划中）*
* 🚧 **`hydra-cli flash-all`** —— 面向控制器和 URTC 刀头的并行固件更新。*（计划中）*
* 🚧 **`hydra-cli audit`** —— 用于 CAN 总线健康状况和传感器验证的自动化诊断套件。*（计划中）*

---

## 2. 🔄 CLI 工作流

```mermaid
flowchart LR
    USER["Developer / DevOps"] --> CLI["HYDRA-UMC-TOOL-CLI"]
    CLI -- HTTP --> SERVER["HYDRA-UMC-SERVER (/api/hydra-info)"]
    SERVER -- Fleet State --> CLI
    CLI -- Result --> USER
```

---

## 3. 🧱 架构与设计决策

* **为何 `src/` 包含一个 `cmd/hydra-cli/` 子路径，而非扁平布局。** 遵循标准的 Go CLI 惯例（一个 `cmd/<binary-name>/` 入口点，并为随着 CLI 超出单一命令规模而增加的未来 `internal/`/`pkg/` 包留出空间）——这并非本生态系统自身的发明，而是更广泛 Go 社区针对多命令 CLI 的自有惯例。
* **为何采用 CLI，而非直接编写脚本调用 HYDRA-UMC-SERVER 自身的 REST API。** 车队规模的操作（跨多个 CM5 进行安装/更新，而非仅一个）需要真正的编排——重试、并行性、一致的用户体验——这些是一次性 curl 脚本无法提供的，HYDRA-UMC-UPDATER 后来在生态系统检出层面应用的正是同样的理由。
* **这如何融入生态系统的其余部分。** 在车队规模上完成 URTC-FLASHER 和 URTC-TESTER 各自为单块板卡所做的事——管理跨车队的多个 HYDRA-UMC-SERVER 实例，而非单块板卡自身的固件。
* **为何 `robots` 读取 `GET /api/settings`，而非新建一个端点。** 该端点已经携带完整的控制器/机器人名单，并且已经是一个真实的、无需鉴权的读取操作（参见 HYDRA-UMC-SERVER 自身的 `src/server.ts`）——`robots` 是一个已发布契约的真实客户端，而非新的服务器端工作。`doctor` 将同一读取与 `/api/hydra-info` 组合，在不添加端点的情况下发现不兼容的公开车队总数。更大型的、仍在计划中的命令（`deploy`/`flash-all`/面向硬件的 `audit`）确实需要目前尚不存在的新写入端点。
* **为何 `doctor` 明确为只读。** 契约检查在硬件到位前很有用，也适合在 CI 中安全运行。它只报告 HTTP/JSON/计数一致性；不会控制设备，也不会声明 CAN、执行器、传感器、摄像头、Hailo、CM5 或安全状态的健康性。
* **为何不带 `--dry-run` 的 `config apply` 会返回“未实现”，而非悄无声息地什么都不做。** 它本应调用的 HYDRA-UMC-SERVER 实时写入端点确实尚不存在（与 `deploy`/`flash-all` 被卡住的原因相同）——一个独立的 `ExitNotImplemented` 退出码会告诉调用者“这是一个真实的缺口，而非 bug”，而不是让其误以为是一次成功的空操作。
* **为何现在每个命令的错误都统一流经一个 `CliError`/`ExitCode` 类型，而非零散的 `os.Exit` 调用。** 一个稳定、有文档记录的退出码契约，只有在存在唯一一处分配退出码的地方时才能保持稳定——`exitCodeFor()`（`exitcode.go`）就是那个地方，各命令函数则继续返回符合惯例、可包装的 `error` 值，而不是自行调用 `os.Exit`。

---

## 📂 目录结构

纯软件 CLI——没有自己的硬件、固件或操作系统；这些目录按照仓库结构策略予以省略。

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Go 模块
│   ├── go.mod                 # 模块定义（github.com/JuanenRac/HYDRA-UMC-TOOL-CLI）
│   └── cmd/hydra-cli/         # 二进制文件入口点
│       ├── main.go            # 命令分发（version/help/status/robots/doctor/config）
│       ├── server.go          # 共享的 --server/HYDRA_CLI_SERVER 解析
│       ├── robots.go          # 真实的 GET /api/settings 客户端 + 名单打印
│       ├── doctor.go          # 只读双端点契约诊断
│       ├── config.go          # 真实的配置文件加载、校验、apply --dry-run
│       ├── exitcode.go        # 真实、稳定的 ExitCode/CliError 契约
│       ├── *_test.go          # 真实测试（net/http/httptest 往返，临时文件测试夹具）
│       └── version.go         # const Version —— 里程表式递增，与清单保持同步
├── docs/                      # 文档：CLI_REFERENCE.md 和 DOCTOR.md
├── build/                     # 编译后的二进制文件（已被 gitignore）
├── images/                    # 媒体与图表
├── bump_version.py            # 原生版本的里程表式递增（由构建运行）
├── bump_manifest_version.py   # 将 hydra-umc.project.json 的版本与原生版本同步(--sync)
├── build.sh / build.bat       # 真实构建：版本递增 + 真实测试套件 + go build + 冒烟测试
├── run.sh / run.bat           # 真实运行：执行编译后的二进制文件
└── README.md
```

---

## 4. ⚙️ 构建与运行

需要 Go >= 1.21。

```bash
# Linux/macOS
./build.sh
./run.sh version
./run.sh status --server http://localhost:3000
./run.sh robots --server http://localhost:3000
./run.sh doctor --server http://localhost:3000
./run.sh config validate --config ./hydra-cli.json
./run.sh config apply --config ./hydra-cli.json --dry-run
echo $?   # 0=正常 2=用法 3=配置 4=网络 5=服务器 6=未实现

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
run.bat doctor --server http://localhost:3000
run.bat config validate --config .\hydra-cli.json
run.bat config apply --config .\hydra-cli.json --dry-run
```

`build` 会递增版本号（`src/cmd/hydra-cli/version.go`），运行真实测试
套件（`go vet` + `go test`），将 `src/` 中的 Go 模块编译为
`build/hydra-cli(.exe)`，并运行一次 `version` 命令以进行验证。`run`
再次执行编译后的二进制文件，并转发所有参数——试着对一个正在运行的
`HYDRA-UMC-SERVER` 实例运行 `run doctor`。Doctor 是安全的只读端点契约检查；详见 [docs/DOCTOR.md](docs/DOCTOR.md)。

---

## 🔗 相关项目

本项目是同一作者(JuanenRac / Electro Hobby 3D)打造的 HYDRA-UMC 机器人生态系统的一部分。值得了解,因为某个请求实际上可能是关于这些项目之一,而非本仓库本身。

**直接相关**
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — 每个控制客户端真正通信的真实无头后端(REST/WebSocket) —— 本 CLI 以车队规模管理的后端。
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — 面向 URTC 板卡的桌面图形烧录工具，支持 CAN-OTA 以及全芯片 SWD/JTAG —— 本工具计划中的车队规模 CAN-OTA 部署,对众多板卡执行的正是 URTC-FLASHER 对单块板卡所做的事情。
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — 面向 URTC 板卡的桌面实时 CAN 总线诊断工具，每种工具配置对应一个面板 —— 本工具计划中的车队规模诊断,对众多板卡执行的正是 URTC-TESTER 对单块板卡所做的事情。

**生态系统中的其他项目**

*核心硬件与平台*
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** — 机器人手臂的真实主板——CM5 主机 + 双核 STM32H745，通过 CAN-OTA/SPI-OTA 协调最多 8 条工具臂。
- **[HYDRA-UMC-OS](https://github.com/JuanenRac/HYDRA-UMC-OS)** — 面向 CM5 的可复现 Raspberry Pi OS 产品层——只读代理、经过验证的配置/配置文件、WiFi 首次配网。
- **[HYDRA-UMC-SDK](https://github.com/JuanenRac/HYDRA-UMC-SDK)** — 每个桥接都据此校验自身指令的共享 JSON-Schema 契约与安全门限边界。

*核心后端与客户端*
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** — 具有实时多机器人 3D 可视化的网页控制面板。
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** — 面向多台服务器的桌面(PySide6)集群指挥中心，打包为独立可执行文件。
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** — 具有生物识别登录和配对 Wear OS 伴侣应用的原生 Android 控制应用。
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** — 具有实时 WebSocket 同步的 iOS/iPadOS 控制应用(Flutter)。
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** — 面向机载 7 英寸 DSI 触摸屏的原生触控界面，直接嵌入 CM5 本体。
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** — 将完成的模型推送到 STUDIO 自身目录的桌面版图形化 URDF 创建/编辑工具。
- **[HYDRA-UMC-BRIDGE-AMR](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-AMR)** — 通过真实的 VDA 5050 MQTT 发布者为 AGV/AMR 车队提供的协调边界。
- **[HYDRA-UMC-BRIDGE-CNC](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-CNC)** — 具备真实 GRBL 状态/控制字节访问能力的高层 CNC 单元协调器。
- **[HYDRA-UMC-BRIDGE-DROIDS](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-DROIDS)** — 面向足式/人形机器人的协调边界，具备真实的 Boston Dynamics Spot 指令发送器。
- **[HYDRA-UMC-BRIDGE-LASER](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-LASER)** — 读取 3 项真实钥匙/外壳/联锁 GPIO 安全信号的激光单元安全协调器。
- **[HYDRA-UMC-BRIDGE-OPENPNP](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-OPENPNP)** — 面向 OpenPnP 贴片机板级流程的安全高层协调器。
- **[HYDRA-UMC-BRIDGE-PRINTER3D](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-PRINTER3D)** — 面向 Moonraker/Klipper 3D 打印机的安全协调边界，具备真实的受控作业指令。
- **[HYDRA-UMC-BRIDGE-ROS2](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-ROS2)** — 具备真实的惰性导入 rclpy ROS 2 传输层的安全协调器。
- **[HYDRA-UMC-BRIDGE-UAV](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-UAV)** — 面向搭载摄像头的无人机的协调边界，具备真实的 MAVLink 指令发送器。

*URTC 工具平台*
- **[URTC](https://github.com/JuanenRac/URTC)** — 面向实体 Universal Robot Tool Controller 板卡的固件，通过 CAN 总线支持 25 种以上工具配置。
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** — 通过 Web Serial API 实现的浏览器版 URTC-TESTER 替代方案，无需本地安装。

*视觉 AI 节点(Hailo-8)*
- **[HYDRA-UMC-VISION-NODE](https://github.com/JuanenRac/HYDRA-UMC-VISION-NODE)** — 面向 Hailo-8 视觉流水线的集成中枢，具备逐阶段的真实硬件就绪检测。
- **[HYDRA-UMC-DETECTION-HEF](https://github.com/JuanenRac/HYDRA-UMC-DETECTION-HEF)** — 具备 Hailo 架构/校验和安全加载验证的真实编译模型注册表。
- **[HYDRA-UMC-VISION-STREAMER](https://github.com/JuanenRac/HYDRA-UMC-VISION-STREAMER)** — 具备真实 HailoRT 集成边界的真实 GStreamer 流水线 + MediaMTX 配置生成器。
- **[HYDRA-UMC-VISUAL-SERVOING-API](https://github.com/JuanenRac/HYDRA-UMC-VISUAL-SERVOING-API)** — 具备真实 Position-Based Visual Servoing 修正律，并依据上游区域状态进行安全门控。
- **[HYDRA-UMC-SAFETY-ZONES](https://github.com/JuanenRac/HYDRA-UMC-SAFETY-ZONES)** — 具备校准新鲜度强制检查的真实区域入侵检测与 E-STOP 请求。

*认知 AI 节点(Hailo-10)*
- **[HYDRA-UMC-COGNITIVE-NODE](https://github.com/JuanenRac/HYDRA-UMC-COGNITIVE-NODE)** — 面向 Hailo-10 认知流水线(LLM/VLA/语音编排)的集成中枢。
- **[HYDRA-UMC-VLA-ENGINE](https://github.com/JuanenRac/HYDRA-UMC-VLA-ENGINE)** — 面向 Vision-Language-Action 模型的真实动作 token 编解码与轨迹生成。
- **[HYDRA-UMC-VOICE-UI](https://github.com/JuanenRac/HYDRA-UMC-VOICE-UI)** — 具备受限、需确认的 Watch 中继的真实语音前端(VAD + 意图解析)。
- **[HYDRA-UMC-SEMANTIC-PLANNER](https://github.com/JuanenRac/HYDRA-UMC-SEMANTIC-PLANNER)** — 基于真实规则的任务分解，以及针对 MCU 错误码的语义化错误恢复。
- **[HYDRA-UMC-DOCS-QA](https://github.com/JuanenRac/HYDRA-UMC-DOCS-QA)** — 面向本生态系统自身 Markdown 文档的真实纯标准库 TF-IDF 文档检索。

*编排与集群*
- **[HYDRA-UMC-ORCHESTRATOR](https://github.com/JuanenRac/HYDRA-UMC-ORCHESTRATOR)** — 具备真实 gRPC/Protobuf 健康报告契约与任务状态机的集成中枢。
- **[HYDRA-UMC-JOB-DISPATCHER](https://github.com/JuanenRac/HYDRA-UMC-JOB-DISPATCHER)** — 基于真实 HTTP API 的真实优先级任务队列，支持去重。
- **[HYDRA-UMC-NODE-HEALING](https://github.com/JuanenRac/HYDRA-UMC-NODE-HEALING)** — 具备重试/退避与身份不匹配检测的真实基于 gRPC 的车队健康看门狗。
- **[HYDRA-UMC-PATH-PLANNER-3D](https://github.com/JuanenRac/HYDRA-UMC-PATH-PLANNER-3D)** — 具备真实障碍物/工作空间碰撞校验的真实基于 RRT 的三维路径规划器。
- **[HYDRA-UMC-SWARM-SYNC](https://github.com/JuanenRac/HYDRA-UMC-SWARM-SYNC)** — 经过多单元收敛属性测试的真实 CRDT LWW-Element-Map 状态同步。

*数字孪生与仿真*
- **[HYDRA-UMC-TWIN](https://github.com/JuanenRac/HYDRA-UMC-TWIN)** — 面向数字孪生引擎的集成中枢，具备真实的版本兼容性同步契约。
- **[HYDRA-UMC-HIL-BRIDGE](https://github.com/JuanenRac/HYDRA-UMC-HIL-BRIDGE)** — 在仿真与真实硬件之间路由指令的真实硬件在环安全联锁。
- **[HYDRA-UMC-PHYSICS-REPLICA](https://github.com/JuanenRac/HYDRA-UMC-PHYSICS-REPLICA)** — 面向真实 URDF 子集的真实正向运动学与关节限位校验。
- **[HYDRA-UMC-SYNTHETIC-DATA-GEN](https://github.com/JuanenRac/HYDRA-UMC-SYNTHETIC-DATA-GEN)** — 具备 YOLO/COCO 标注导出功能的真实程序化 2D 场景生成器。

*数据与分析*
- **[HYDRA-UMC-DATALAKE](https://github.com/JuanenRac/HYDRA-UMC-DATALAKE)** — 具备真实数据摄入/查询 HTTP API 的真实 sqlite3 时序数据存储。
- **[HYDRA-UMC-ANOMALY-DETECTOR](https://github.com/JuanenRac/HYDRA-UMC-ANOMALY-DETECTOR)** — 具备漂移监测能力的真实 FFT + 统计基线异常检测器。
- **[HYDRA-UMC-PRODUCTION-REPORTS](https://github.com/JuanenRac/HYDRA-UMC-PRODUCTION-REPORTS)** — 基于 DATALAKE 历史数据的真实 OEE/可用率计算，支持可复现的 CSV 导出。
- **[HYDRA-UMC-TELEMETRY-COLLECTOR](https://github.com/JuanenRac/HYDRA-UMC-TELEMETRY-COLLECTOR)** — 面向 DATALAKE 的真实 CAN/WebSocket 数据摄入管道，支持序列去重。

*工业网关*
- **[HYDRA-UMC-GATEWAY-INDUSTRIAL](https://github.com/JuanenRac/HYDRA-UMC-GATEWAY-INDUSTRIAL)** — 中继至工业协议的集成中枢，具备真实的指令白名单/背压控制层。
- **[HYDRA-UMC-OPCUA-SERVER](https://github.com/JuanenRac/HYDRA-UMC-OPCUA-SERVER)** — 经真实二进制协议客户端会话验证的真实 OPC-UA 地址空间。
- **[HYDRA-UMC-MQTT-BROKER](https://github.com/JuanenRac/HYDRA-UMC-MQTT-BROKER)** — 具备可选按客户端认证与主题 ACL 的真实 MQTT 代理。
- **[HYDRA-UMC-MTCONNECT-ADAPTER](https://github.com/JuanenRac/HYDRA-UMC-MTCONNECT-ADAPTER)** — 具备降级模式输出的真实 MTConnect `/probe` 与 `/current` XML 端点。

*辅助工具与生态系统运维*
- **[HYDRA-UMC-DASHBOARD-AI](https://github.com/JuanenRac/HYDRA-UMC-DASHBOARD-AI)** — 基于 DATALAKE/ANOMALY-DETECTOR 的智能摘要与异常高亮面板，具备诚实的统计回退机制。
- **[HYDRA-UMC-WATCH](https://github.com/JuanenRac/HYDRA-UMC-WATCH)** — 具备真实触觉提醒与配对手机语音中继功能的 WearOS 伴侣应用。
- **[URTC-SMART-RACK](https://github.com/JuanenRac/URTC-SMART-RACK)** — 面向板卡安装机架的固件，具备真实的工具 ID 解码与 Smart Idle 预热逻辑。
- **[URTC-VISION-TOOL](https://github.com/JuanenRac/URTC-VISION-TOOL)** — 面向热成像/RGB 检测工具头的固件及真实 Python 视觉伴侣程序。
- **[HYDRA-UMC-UPDATER](https://github.com/JuanenRac/HYDRA-UMC-UPDATER)** — 发现、克隆并更新本生态系统中每个仓库的管理类桌面工具。


---

## 📚 文档与社区

- **[CONTRIBUTING.md](CONTRIBUTING.md)** —— 提交 Pull Request 所需的技术栈和编码规范。
- **[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)** —— 本社区所期望的行为准则。
- **[SECURITY.md](SECURITY.md)** —— 如何报告漏洞，以及本项目真实的安全关注重点。
- **[SUPPORT.md](SUPPORT.md)** —— 在哪里提问和报告缺陷。
- **[LICENSE.md](LICENSE.md)** —— 本项目自身的许可证。

## 👤 作者
**JuanenRac** (Electro Hobby 3D)
📧 electrohobby3d@gmail.com
📺 [youtube.com/@electrohobby3d](https://youtube.com/@electrohobby3d)

## 📜 许可证
GPL-3.0 —— 详见 LICENSE。
