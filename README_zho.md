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
* **为何入口点今天只打印身份/版本/角色。** 处于脚手架（scaffolding）阶段：证明 `go build ./cmd/hydra-cli` 成功，先于真正的车队管理命令集。
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
│       └── version.go         # const Version = "0.0.0"
├── docs/                      # 文档：CLI_REFERENCE.md 和 DOCTOR.md
├── build/                     # 编译后的二进制文件（已被 gitignore）
├── images/                    # 媒体与图表
├── scripts/                   # 实用脚本
├── bump_version.py            # 里程表式版本递增（由构建运行）
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

本项目是同一作者（JuanenRac / Electro Hobby 3D）打造的更大规模机器人生态
系统的一部分，涵盖固件、控制软件、AI 节点和车队工具。值得了解，因为某个
需求实际上可能是关于这些项目之一，而非本仓库。

### 直接相关

- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** —— 本 CLI 在车队规模上所管理的后端。
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** —— 在车队规模上完成本工具为单块板卡所做的事。
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** —— 在车队规模上完成本工具为单块板卡所做的事。

### 生态系统的其余部分

**HYDRA-UMC 平台** —— 多机器人微工厂单元
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** —— 协调最多 8 条机械臂的 CM5 + STM32H745 主板。
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** —— 每个控制客户端所对接的 Express/WebSocket 后端。
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** —— 基于 Web 的控制仪表盘，多机器人 3D 可视化。
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** —— 通过 Wi-Fi/蓝牙的 Android 控制应用。
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** —— 基于 Flutter 构建的 iOS/iPadOS 控制应用。
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** —— 桌面端集群指挥中心（Python/PySide6）。
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** —— 用于机器人目录的桌面端 URDF 模型编辑器。
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** —— 机载 DSI 触摸屏的原生触控 UI。

**URTC 平台** —— 每台 HYDRA-UMC 机械臂搭载的工具头控制器
- **[URTC](https://github.com/JuanenRac/URTC)** —— CAN 总线工具头控制器，25 种工具配置。
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** —— 桌面端 CAN-OTA + SWD/JTAG 刷写工具。
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** —— 桌面端实时 CAN 总线诊断工具。
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** —— 通过 Web Serial API 的浏览器端替代方案。

**🎥 视觉 AI 节点（Hailo-8）**
- [HYDRA-UMC-VISION-NODE](https://github.com/JuanenRac/HYDRA-UMC-VISION-NODE)
- [HYDRA-UMC-VISION-STREAMER](https://github.com/JuanenRac/HYDRA-UMC-VISION-STREAMER)
- [HYDRA-UMC-DETECTION-HEF](https://github.com/JuanenRac/HYDRA-UMC-DETECTION-HEF)
- [HYDRA-UMC-SAFETY-ZONES](https://github.com/JuanenRac/HYDRA-UMC-SAFETY-ZONES)
- [HYDRA-UMC-VISUAL-SERVOING-API](https://github.com/JuanenRac/HYDRA-UMC-VISUAL-SERVOING-API)

**🧠 认知 AI 节点（Hailo-10）**
- [HYDRA-UMC-COGNITIVE-NODE](https://github.com/JuanenRac/HYDRA-UMC-COGNITIVE-NODE)
- [HYDRA-UMC-VLA-ENGINE](https://github.com/JuanenRac/HYDRA-UMC-VLA-ENGINE)
- [HYDRA-UMC-VOICE-UI](https://github.com/JuanenRac/HYDRA-UMC-VOICE-UI)
- [HYDRA-UMC-SEMANTIC-PLANNER](https://github.com/JuanenRac/HYDRA-UMC-SEMANTIC-PLANNER)
- [HYDRA-UMC-DOCS-QA](https://github.com/JuanenRac/HYDRA-UMC-DOCS-QA)

**🐝 编排与集群**
- [HYDRA-UMC-ORCHESTRATOR](https://github.com/JuanenRac/HYDRA-UMC-ORCHESTRATOR)
- [HYDRA-UMC-SWARM-SYNC](https://github.com/JuanenRac/HYDRA-UMC-SWARM-SYNC)
- [HYDRA-UMC-PATH-PLANNER-3D](https://github.com/JuanenRac/HYDRA-UMC-PATH-PLANNER-3D)
- [HYDRA-UMC-JOB-DISPATCHER](https://github.com/JuanenRac/HYDRA-UMC-JOB-DISPATCHER)
- [HYDRA-UMC-NODE-HEALING](https://github.com/JuanenRac/HYDRA-UMC-NODE-HEALING)

**🎮 数字孪生与仿真**
- [HYDRA-UMC-TWIN](https://github.com/JuanenRac/HYDRA-UMC-TWIN)
- [HYDRA-UMC-PHYSICS-REPLICA](https://github.com/JuanenRac/HYDRA-UMC-PHYSICS-REPLICA)
- [HYDRA-UMC-HIL-BRIDGE](https://github.com/JuanenRac/HYDRA-UMC-HIL-BRIDGE)
- [HYDRA-UMC-SYNTHETIC-DATA-GEN](https://github.com/JuanenRac/HYDRA-UMC-SYNTHETIC-DATA-GEN)

**📊 数据与分析**
- [HYDRA-UMC-DATALAKE](https://github.com/JuanenRac/HYDRA-UMC-DATALAKE)
- [HYDRA-UMC-TELEMETRY-COLLECTOR](https://github.com/JuanenRac/HYDRA-UMC-TELEMETRY-COLLECTOR)
- [HYDRA-UMC-ANOMALY-DETECTOR](https://github.com/JuanenRac/HYDRA-UMC-ANOMALY-DETECTOR)
- [HYDRA-UMC-PRODUCTION-REPORTS](https://github.com/JuanenRac/HYDRA-UMC-PRODUCTION-REPORTS)

**🏭 工业网关**
- [HYDRA-UMC-GATEWAY-INDUSTRIAL](https://github.com/JuanenRac/HYDRA-UMC-GATEWAY-INDUSTRIAL)
- [HYDRA-UMC-OPCUA-SERVER](https://github.com/JuanenRac/HYDRA-UMC-OPCUA-SERVER)
- [HYDRA-UMC-MQTT-BROKER](https://github.com/JuanenRac/HYDRA-UMC-MQTT-BROKER)
- [HYDRA-UMC-MTCONNECT-ADAPTER](https://github.com/JuanenRac/HYDRA-UMC-MTCONNECT-ADAPTER)

**🛠️ 配套工具**
- [URTC-SMART-RACK](https://github.com/JuanenRac/URTC-SMART-RACK)
- [URTC-VISION-TOOL](https://github.com/JuanenRac/URTC-VISION-TOOL)
- [HYDRA-UMC-WATCH](https://github.com/JuanenRac/HYDRA-UMC-WATCH)
- [HYDRA-UMC-DASHBOARD-AI](https://github.com/JuanenRac/HYDRA-UMC-DASHBOARD-AI)


## 👤 作者
**JuanenRac**（Electro Hobby 3D）
📧 electrohobby3d@gmail.com

## 📜 许可证
GPL-3.0 —— 详见 LICENSE。

## 🛠️ BUILD & RUN

请在发布构建前使用不改动版本的构建检查：

| 操作 | Windows | Linux / macOS |
|---|---|---|
| 构建检查（不修改版本或 CHANGELOG） | `build-test.bat` | `./build-test.sh` |
| 运行 / 开发（如提供） | `run*.bat` 或 `dev*.bat` | `./run*.sh` 或 `./dev*.sh` |

`build-test.bat` 和 `build-test.sh` 会编译或验证项目技术栈，但不会递增 `hydra-umc.project.json`，也不会修改 `CHANGELOG.md`。它们仅可能生成正常的编译器输出。现有的 `build*.bat`、`build*.sh`、`run*` 和 `dev*` 脚本保留各自的版本化或运行时行为；需要该行为时请使用它们。
