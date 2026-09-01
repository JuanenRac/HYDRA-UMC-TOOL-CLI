<p align="center">
  <img src="images/HYDRA_UMC_BANNER.svg" alt="HYDRA-UMC-TOOL-CLI banner" width="100%">
</p>

# 💻 HYDRA-UMC-TOOL-CLI

<p align="center"><a href="README.md">🇺🇸 English</a> | <a href="README_spa.md">🇪🇸 Español</a> | <a href="README_fra.md">🇫🇷 Français</a> | <a href="README_ita.md">🇮🇹 Italiano</a> | <a href="README_deu.md">🇩🇪 Deutsch</a> | <a href="README_zho.md">🇨🇳 简体中文</a> | 🇯🇵 <b>日本語</b></p>

### 🛠️ フリート DevOps と自動化のためのコマンドラインインターフェース

<p align="left">
  <img src="https://img.shields.io/badge/Licencia-GPL%203.0-blue.svg" alt="GPL 3.0">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Feature-Fleet%20DevOps-blue.svg" alt="DevOps">
</p>

---

## 1. 🛠️ 技術概要

**HYDRA-UMC-TOOL-CLI** は、HYDRA-UMC エコシステムの開発者およびシステム
管理者向けの十徳ナイフです。HYDRA-UMC デプロイメントを照会、更新、
監査するためのコマンドラインツールを提供する、単一の静的な Go バイナリ
です。

長期的な目標は、ターミナルまたは CI/CD パイプラインから直接、大規模な
ミッションのデプロイ、並列ファームウェア更新（CAN-OTA）、そして詳細な
システム診断を行うことです。今日の時点では、他のすべての機能が基盤と
する実際に動作する基礎——バージョンレポートと HYDRA-UMC-SERVER に対する
ライブ HTTP クライアント——が提供されています。

### 主な機能：
* ✅ **`hydra-cli version`** — CLI 自身の名前とバージョンを表示します。*（実装済み）*
* ✅ **`hydra-cli status [--server URL]`** — 実行中の HYDRA-UMC-SERVER の `GET /api/hydra-info` を照会し、報告されたアイデンティティを表示します。*（実装済み）*
* ✅ **`hydra-cli robots [--server URL]`** — 実行中の HYDRA-UMC-SERVER の `GET /api/settings` を照会し、実際のコントローラー/ロボット一覧（名前、オンライン状態、モデル、役割）を表示します。*（実装済み）*
* ✅ **`hydra-cli doctor [--server URL]`** — 読み取り専用のサーバー契約診断：`/api/hydra-info` と `/api/settings` を検証し、公開されたコントローラー/ロボット数を一覧と照合します。コマンド送信やハードウェアの検査は行いません。*（実装済み）*
* ✅ **実際に安定した終了コード契約** — `0` 正常、`1` 一般エラー、`2` 使用方法エラー、`3` 設定エラー、`4` ネットワークエラー、`5` サーバーエラー、`6` 未実装。各コマンドは単なる `exit 1` の代わりにこの契約を通じて自身の失敗を分類するため、この CLI をラップするスクリプトは*失敗の理由*に応じて分岐できます。*（実装済み）*
* ✅ **`hydra-cli config validate --config PATH`** — ローカルの設定ファイル（サーバー URL、リクエストタイムアウト）を読み込み、スキーマ検証します。*（実装済み）*
* ✅ **`hydra-cli config apply --config PATH [--dry-run]`** — `--dry-run` は実際の検証パスをエンドツーエンドで証明し、送信される内容を正確に表示します。指定しない場合は、実際にはまだライブのフリート書き込みエンドポイントが存在しないため、正直に「未実装」を返します。*（実装済み、dry-run のみ）*
* ✅ **`hydra-cli shell [--server URL]`** — 対話型 REPL:プロセスを再起動せずに、上記のいずれのコマンドも同じサーバーに対して繰り返し実行できます。単発実行と全く同じコマンドテーブルを介して処理されるため、シェルと単発実行の挙動が食い違うことはありません。`exit`/`quit`/Ctrl-D で終了。*（実装済み）*
* ✅ **`hydra-cli help` / `--help`** — 完全なコマンド使用方法。*（実装済み）*
* 🚧 **`hydra-cli deploy`** — ミッションと設定を複数のロボットに同時にアップロードします。*（計画中）*
* 🚧 **`hydra-cli flash-all`** — コントローラーと URTC ヘッド向けの並列ファームウェア更新。*（計画中）*
* 🚧 **`hydra-cli audit`** — CAN バスの健全性とセンサー検証のための自動診断スイート。*（計画中）*

---

## 2. 🔄 CLI ワークフロー

```mermaid
flowchart LR
    USER["Developer / DevOps"] --> CLI["HYDRA-UMC-TOOL-CLI"]
    CLI -- HTTP --> SERVER["HYDRA-UMC-SERVER (/api/hydra-info)"]
    SERVER -- Fleet State --> CLI
    CLI -- Result --> USER
```

---

## 3. 🧱 アーキテクチャと設計上の決定

* **`src/` がフラットなレイアウトではなく `cmd/hydra-cli/` サブパスを持つ理由。** 標準的な Go CLI の慣例（`cmd/<binary-name>/` エントリポイント、CLI が単一コマンドを超えて成長した際の将来の `internal/`/`pkg/` パッケージのための余地）に従っています——これはこのエコシステム独自の発明ではなく、マルチコマンド CLI に対する Go コミュニティ全体の慣例です。
* **HYDRA-UMC-SERVER 自身の REST API に直接スクリプトを書くのではなく、CLI を採用した理由。** フリート規模の操作（1 台だけでなく多数の CM5 にわたるインストール/更新）には、リトライ、並列性、一貫した UX といった、その場しのぎの curl スクリプトでは提供できない真のオーケストレーションが必要です。これは、後に HYDRA-UMC-UPDATER がエコシステムチェックアウトのレベルで適用するのと同じ理由です。
* **エントリポイントが今日は身元/バージョン/役割のみを表示する理由。** 足場（アンダミアヘ、スキャフォールディング）段階にあります：`go build ./cmd/hydra-cli` が成功することを証明することが、実際のフリート管理コマンドセットに先立ちます。
* **エコシステムの他の部分との関係。** URTC-FLASHER と URTC-TESTER がそれぞれ 1 枚の基板に対して行っていることを、フリート規模で行います——単一基板自身のファームウェアではなく、フリート全体にわたる複数の HYDRA-UMC-SERVER インスタンスを管理します。
* **`robots` が新しいエンドポイントではなく `GET /api/settings` を読み取る理由。** そのエンドポイントはすでにコントローラー/ロボットの完全な一覧を持っており、すでに実際の、認証不要の読み取り操作です（HYDRA-UMC-SERVER 自身の `src/server.ts` を参照）——`robots` はすでに提供されている契約に対する実際のクライアントであり、新たなサーバー側の作業ではありません。`doctor` は同じ読み取りを `/api/hydra-info` と組み合わせ、新しいエンドポイントを追加せずに不整合な公開フリート数を検出します。より大規模な、まだ計画段階のコマンド（`deploy`/`flash-all`/ハードウェア向け `audit`）は、まだ存在しない新しい書き込みエンドポイントを本当に必要とします。
* **`doctor` が明示的に読み取り専用である理由。** 契約チェックはハードウェアがない段階でも有用で、CI でも安全です。報告するのは HTTP/JSON/数の整合性だけであり、機器を操作したり、CAN、アクチュエータ、センサー、カメラ、Hailo、CM5、安全性の健全性を主張したりしません。
* **`--dry-run` を指定しない `config apply` が、黙って何もしない代わりに「未実装」を返す理由。** これが呼び出すはずの HYDRA-UMC-SERVER 上のライブ書き込みエンドポイントは、実際にはまだ存在しません（`deploy`/`flash-all` が塞き止められているのと同じギャップです）——専用の `ExitNotImplemented` 終了コードは、呼び出し元に「これは本物のギャップであり、バグではない」ことを伝えます。誤解を招くような、成功したように見える無操作にはしません。
* **すべてのコマンドのエラーが、場当たり的な `os.Exit` 呼び出しではなく、単一の `CliError`/`ExitCode` 型を通じて流れるようになった理由。** 安定した、文書化された終了コード契約は、コードを割り当てる場所が一つしかない場合にのみ安定を保てます——`exitCodeFor()`（`exitcode.go`）がその場所であり、各コマンド関数は自ら `os.Exit` を呼び出す代わりに、慣用的でラップ可能な `error` 値を返し続けます。

---

## 📂 リポジトリ構成

純粋なソフトウェア CLI——独自のハードウェア/ファームウェア/OS を持たず、
テンプレートから省略されており、リポジトリ構造ポリシーに従っています。

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Go モジュール
│   ├── go.mod                 # モジュール定義（github.com/JuanenRac/HYDRA-UMC-TOOL-CLI）
│   └── cmd/hydra-cli/         # バイナリのエントリポイント
│       ├── main.go            # コマンドディスパッチ（version/help/status/robots/doctor/config）
│       ├── server.go          # --server/HYDRA_CLI_SERVER の共有解決処理
│       ├── robots.go          # 実際の GET /api/settings クライアント + 一覧表示
│       ├── doctor.go          # 読み取り専用の二つのエンドポイント契約診断
│       ├── config.go          # 実際の設定ファイルの読み込み、検証、apply --dry-run
│       ├── exitcode.go        # 実際の、安定した ExitCode/CliError 契約
│       ├── *_test.go          # 実際のテスト（net/http/httptest ラウンドトリップ、一時ファイルのフィクスチャ）
│       └── version.go         # const Version = "0.0.0"
├── docs/                      # ドキュメント：CLI_REFERENCE.md と DOCTOR.md
├── build/                     # コンパイル済みバイナリ（gitignore 対象）
├── images/                    # メディアと図表
├── scripts/                   # ユーティリティスクリプト
├── bump_version.py            # オドメーター式バージョンインクリメント（ビルドが実行）
├── build.sh / build.bat       # 実際のビルド：バージョンインクリメント + 実際のテストスイート + go build + スモークテスト
├── run.sh / run.bat           # 実際の実行：コンパイル済みバイナリを実行
└── README.md
```

---

## 4. ⚙️ ビルドと実行

Go >= 1.21 が必要です。

```bash
# Linux/macOS
./build.sh
./run.sh version
./run.sh status --server http://localhost:3000
./run.sh robots --server http://localhost:3000
./run.sh doctor --server http://localhost:3000
./run.sh config validate --config ./hydra-cli.json
./run.sh config apply --config ./hydra-cli.json --dry-run
echo $?   # 0=正常 2=使用方法 3=設定 4=ネットワーク 5=サーバー 6=未実装

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
run.bat doctor --server http://localhost:3000
run.bat config validate --config .\hydra-cli.json
run.bat config apply --config .\hydra-cli.json --dry-run
```

`build` はバージョンを増加させ（`src/cmd/hydra-cli/version.go`）、
実際のテストスイートを実行し（`go vet` + `go test`）、`src/` 内の Go
モジュールを `build/hydra-cli(.exe)` としてコンパイルし、検証のために
一度 `version` を実行します。`run` はコンパイル済みバイナリを再度実行
し、すべての引数を転送します——実行中の `HYDRA-UMC-SERVER` インスタンス
に対して `run doctor` を試してみてください。Doctor は安全な読み取り専用のエンドポイント契約チェックです。詳細は [docs/DOCTOR.md](docs/DOCTOR.md) を参照してください。

---

## 🔗 関連プロジェクト

本プロジェクトは、同一著者（JuanenRac / Electro Hobby 3D）による、
ファームウェア、制御ソフトウェア、AI ノード、フリート管理ツールにまたがる、
より大きなロボティクスエコシステムの一部です。ご要望が実際にはこれらの
プロジェクトのいずれかに関するものであり、本リポジトリのものではない
可能性もあるため、知っておく価値があります。

### 直接関連

- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** —— 本 CLI がフリート規模で管理するバックエンド。
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** —— 本ツールが 1 枚の基板に対して行うことを、フリート規模で行います。
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** —— 本ツールが 1 枚の基板に対して行うことを、フリート規模で行います。

### エコシステムのその他のプロジェクト

**HYDRA-UMC プラットフォーム** — マルチロボット・マイクロファクトリーセル
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** — 最大 8 台のロボットアームを統括する CM5 + STM32H745 マザーボード。
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — すべての制御クライアントが接続する Express/WebSocket バックエンド。
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** — Web ベースの制御ダッシュボード、マルチロボット 3D 可視化。
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** — Wi-Fi/Bluetooth 経由の Android 制御アプリ。
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** — Flutter で構築された iOS/iPadOS 制御アプリ。
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** — デスクトップ版群制御コマンドセンター（Python/PySide6）。
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** — ロボットカタログ向けのデスクトップ版 URDF モデルエディター。
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** — 機載 DSI タッチスクリーン用のネイティブタッチ UI。

**URTC プラットフォーム** — すべての HYDRA-UMC ロボットアームが搭載するツールヘッドコントローラー
- **[URTC](https://github.com/JuanenRac/URTC)** — CAN バスツールヘッドコントローラー、25 種類のツールプロファイル。
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — デスクトップ版 CAN-OTA + SWD/JTAG フラッシュツール。
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — デスクトップ版ライブ CAN バス診断ツール。
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** — Web Serial API によるブラウザベースの代替版。

**🎥 ビジョン AI ノード（Hailo-8）**
- [HYDRA-UMC-VISION-NODE](https://github.com/JuanenRac/HYDRA-UMC-VISION-NODE)
- [HYDRA-UMC-VISION-STREAMER](https://github.com/JuanenRac/HYDRA-UMC-VISION-STREAMER)
- [HYDRA-UMC-DETECTION-HEF](https://github.com/JuanenRac/HYDRA-UMC-DETECTION-HEF)
- [HYDRA-UMC-SAFETY-ZONES](https://github.com/JuanenRac/HYDRA-UMC-SAFETY-ZONES)
- [HYDRA-UMC-VISUAL-SERVOING-API](https://github.com/JuanenRac/HYDRA-UMC-VISUAL-SERVOING-API)

**🧠 認知 AI ノード（Hailo-10）**
- [HYDRA-UMC-COGNITIVE-NODE](https://github.com/JuanenRac/HYDRA-UMC-COGNITIVE-NODE)
- [HYDRA-UMC-VLA-ENGINE](https://github.com/JuanenRac/HYDRA-UMC-VLA-ENGINE)
- [HYDRA-UMC-VOICE-UI](https://github.com/JuanenRac/HYDRA-UMC-VOICE-UI)
- [HYDRA-UMC-SEMANTIC-PLANNER](https://github.com/JuanenRac/HYDRA-UMC-SEMANTIC-PLANNER)
- [HYDRA-UMC-DOCS-QA](https://github.com/JuanenRac/HYDRA-UMC-DOCS-QA)

**🐝 オーケストレーションと群制御**
- [HYDRA-UMC-ORCHESTRATOR](https://github.com/JuanenRac/HYDRA-UMC-ORCHESTRATOR)
- [HYDRA-UMC-SWARM-SYNC](https://github.com/JuanenRac/HYDRA-UMC-SWARM-SYNC)
- [HYDRA-UMC-PATH-PLANNER-3D](https://github.com/JuanenRac/HYDRA-UMC-PATH-PLANNER-3D)
- [HYDRA-UMC-JOB-DISPATCHER](https://github.com/JuanenRac/HYDRA-UMC-JOB-DISPATCHER)
- [HYDRA-UMC-NODE-HEALING](https://github.com/JuanenRac/HYDRA-UMC-NODE-HEALING)

**🎮 デジタルツインとシミュレーション**
- [HYDRA-UMC-TWIN](https://github.com/JuanenRac/HYDRA-UMC-TWIN)
- [HYDRA-UMC-PHYSICS-REPLICA](https://github.com/JuanenRac/HYDRA-UMC-PHYSICS-REPLICA)
- [HYDRA-UMC-HIL-BRIDGE](https://github.com/JuanenRac/HYDRA-UMC-HIL-BRIDGE)
- [HYDRA-UMC-SYNTHETIC-DATA-GEN](https://github.com/JuanenRac/HYDRA-UMC-SYNTHETIC-DATA-GEN)

**📊 データと分析**
- [HYDRA-UMC-DATALAKE](https://github.com/JuanenRac/HYDRA-UMC-DATALAKE)
- [HYDRA-UMC-TELEMETRY-COLLECTOR](https://github.com/JuanenRac/HYDRA-UMC-TELEMETRY-COLLECTOR)
- [HYDRA-UMC-ANOMALY-DETECTOR](https://github.com/JuanenRac/HYDRA-UMC-ANOMALY-DETECTOR)
- [HYDRA-UMC-PRODUCTION-REPORTS](https://github.com/JuanenRac/HYDRA-UMC-PRODUCTION-REPORTS)

**🏭 産業用ゲートウェイ**
- [HYDRA-UMC-GATEWAY-INDUSTRIAL](https://github.com/JuanenRac/HYDRA-UMC-GATEWAY-INDUSTRIAL)
- [HYDRA-UMC-OPCUA-SERVER](https://github.com/JuanenRac/HYDRA-UMC-OPCUA-SERVER)
- [HYDRA-UMC-MQTT-BROKER](https://github.com/JuanenRac/HYDRA-UMC-MQTT-BROKER)
- [HYDRA-UMC-MTCONNECT-ADAPTER](https://github.com/JuanenRac/HYDRA-UMC-MTCONNECT-ADAPTER)

**🛠️ 補完ツール**
- [URTC-SMART-RACK](https://github.com/JuanenRac/URTC-SMART-RACK)
- [URTC-VISION-TOOL](https://github.com/JuanenRac/URTC-VISION-TOOL)
- [HYDRA-UMC-WATCH](https://github.com/JuanenRac/HYDRA-UMC-WATCH)
- [HYDRA-UMC-DASHBOARD-AI](https://github.com/JuanenRac/HYDRA-UMC-DASHBOARD-AI)


## 👤 作者
**JuanenRac**（Electro Hobby 3D）
📧 electrohobby3d@gmail.com

## 📜 ライセンス
GPL-3.0 —— 詳細は LICENSE を参照してください。
