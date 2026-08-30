<p align="center">
  <img src="images/HYDRA_UMC_BANNER.svg" alt="HYDRA-UMC-TOOL-CLI banner" width="100%">
</p>

# 💻 HYDRA-UMC-TOOL-CLI

<p align="center"><a href="README.md">🇺🇸 English</a> | <a href="README_spa.md">🇪🇸 Español</a> | <a href="README_fra.md">🇫🇷 Français</a> | <a href="README_ita.md">🇮🇹 Italiano</a> | 🇩🇪 <b>Deutsch</b> | <a href="README_zho.md">🇨🇳 简体中文</a> | <a href="README_jpn.md">🇯🇵 日本語</a></p>

### 🛠️ Kommandozeilen-Schnittstelle für Flotten-DevOps & Automatisierung

<p align="left">
  <img src="https://img.shields.io/badge/Licencia-GPL%203.0-blue.svg" alt="GPL 3.0">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Feature-Fleet%20DevOps-blue.svg" alt="DevOps">
</p>

---

## 1. 🛠️ TECHNISCHER ÜBERBLICK

**HYDRA-UMC-TOOL-CLI** ist das Schweizer Taschenmesser für Entwickler und Systemadministratoren des HYDRA-UMC-Ökosystems. Es ist eine einzelne statische Go-Binärdatei, die Kommandozeilenwerkzeuge zum Abfragen, Aktualisieren und Prüfen von HYDRA-UMC-Deployments bereitstellt.

Das langfristige Ziel sind massive Missionsausrollungen, parallele Firmware-Updates (CAN-OTA) und tiefgehende Systemdiagnosen direkt aus einem Terminal oder einer CI/CD-Pipeline. Heute enthält es das reale, funktionierende Fundament, auf dem alles Weitere aufbaut: Versionsauskunft und einen echten HTTP-Client gegen HYDRA-UMC-SERVER.

### Hauptmerkmale:
* ✅ **`hydra-cli version`** — gibt Name und Version des CLI selbst aus. *(implementiert)*
* ✅ **`hydra-cli status [--server URL]`** — fragt `GET /api/hydra-info` einer laufenden HYDRA-UMC-SERVER-Instanz ab und gibt deren gemeldete Identität aus. *(implementiert)*
* ✅ **`hydra-cli robots [--server URL]`** — fragt `GET /api/settings` einer laufenden HYDRA-UMC-SERVER-Instanz ab und gibt deren echte Controller-/Roboterliste aus (Name, Online-Status, Modell, Rolle). *(implementiert)*
* ✅ **`hydra-cli doctor [--server URL]`** — schreibgeschützte Server-Vertragsdiagnose: prüft `/api/hydra-info` und `/api/settings` und vergleicht veröffentlichte Controller-/Roboterzahlen mit der Liste. Sendet keine Befehle und prüft keine Hardware. *(implementiert)*
* ✅ **Ein echter, stabiler Exit-Code-Vertrag** — `0` ok, `1` allgemeiner Fehler, `2` Nutzungsfehler, `3` Konfigurationsfehler, `4` Netzwerkfehler, `5` Serverfehler, `6` nicht implementiert. Jeder Befehl klassifiziert seine eigenen Fehlschläge über diesen Vertrag statt über ein bloßes `exit 1`, sodass Skripte, die dieses CLI umschließen, anhand des *Warum* eines Fehlschlags verzweigen können. *(implementiert)*
* ✅ **`hydra-cli config validate --config PATH`** — lädt und validiert eine lokale Konfigurationsdatei anhand eines Schemas (Server-URL, Anfrage-Timeout). *(implementiert)*
* ✅ **`hydra-cli config apply --config PATH [--dry-run]`** — `--dry-run` beweist den echten Validierungspfad end-to-end und gibt genau aus, was gesendet würde; ohne diese Option liefert der Befehl ehrlich "nicht implementiert" zurück, da es noch keinen Live-Schreib-Endpunkt für die Flotte gibt. *(implementiert, nur dry-run)*
* ✅ **`hydra-cli help` / `--help`** — vollständige Befehlsübersicht. *(implementiert)*
* 🚧 **`hydra-cli deploy`** — lädt Missionen und Konfigurationen gleichzeitig auf eine Roboterflotte hoch. *(geplant)*
* 🚧 **`hydra-cli flash-all`** — parallele Firmware-Updates für Controller und URTC-Köpfe. *(geplant)*
* 🚧 **`hydra-cli audit`** — automatisierte Diagnosesuite für CAN-Bus-Gesundheit und Sensorvalidierung. *(geplant)*

---

## 2. 🔄 CLI-WORKFLOW

```mermaid
flowchart LR
    USER["Entwickler / DevOps"] --> CLI["HYDRA-UMC-TOOL-CLI"]
    CLI -- HTTP --> SERVER["HYDRA-UMC-SERVER (/api/hydra-info)"]
    SERVER -- Flottenstatus --> CLI
    CLI -- Ergebnis --> USER
```

---

## 3. 🧱 ARCHITEKTUR & DESIGNENTSCHEIDUNGEN

* **Warum `src/` einen Unterpfad `cmd/hydra-cli/` enthält, kein flaches Layout.** Folgt der Standard-Go-Konvention für CLIs (ein Einstiegspunkt `cmd/<Binärname>/`, mit Platz für künftige `internal/`/`pkg/`-Pakete, sobald die CLI über einen einzelnen Befehl hinauswächst) - keine Erfindung dieses Ökosystems, sondern die eigene Konvention der breiteren Go-Community für Multi-Befehl-CLIs.
* **Warum eine CLI und nicht einfach direktes Scripting gegen die eigene REST-API von HYDRA-UMC-SERVER.** Operationen im Flottenmaßstab (Installieren/Aktualisieren über viele CM5, nicht nur eines) brauchen echte Orchestrierung - Wiederholungen, Parallelität, eine konsistente UX -, die ein einmaliges curl-Skript nicht bietet, derselbe Grund, den HYDRA-UMC-UPDATER später auf Ebene des gesamten Ökosystem-Checkouts anwendet.
* **Warum der Einstiegspunkt heute nur Identität/Version/Rolle ausgibt.** Andamiaje-Stadium: der Nachweis, dass `go build ./cmd/hydra-cli` gelingt, geht dem echten Flottenverwaltungs-Befehlssatz voraus.
* **Wie sich das ins restliche Ökosystem einfügt.** Macht im Flottenmaßstab, was URTC-FLASHER und URTC-TESTER jeweils für eine einzelne Platine machen - verwaltet HYDRA-UMC-SERVER-Instanzen über eine Flotte hinweg statt der eigenen Firmware einer einzelnen Platine.
* **Warum `robots` `GET /api/settings` liest statt eines neuen Endpunkts.** Dieser Endpunkt liefert bereits die vollständige Controller-/Roboterliste und ist bereits eine echte, unauthentifizierte Lese-Operation (siehe das eigene `src/server.ts` von HYDRA-UMC-SERVER) - `robots` ist ein echter Client eines bereits ausgelieferten Vertrags, keine neue serverseitige Arbeit. `doctor` nutzt dieselbe Lese-Operation zusammen mit `/api/hydra-info`, um inkompatible öffentliche Flottenzahlen ohne neuen Endpunkt zu erkennen. Die größeren, noch geplanten Befehle (`deploy`/`flash-all`/hardwarebezogenes `audit`) benötigen wirklich neue Schreib-Endpunkte, die noch nicht existieren.
* **Warum `doctor` ausdrücklich schreibgeschützt ist.** Eine Vertragsprüfung ist vor vorhandener Hardware nützlich und in CI sicher. Sie meldet nur HTTP-/JSON-/Zählerkonsistenz; sie steuert keine Geräte und behauptet keine CAN-, Aktor-, Sensor-, Kamera-, Hailo-, CM5- oder Sicherheitsgesundheit.
* **Warum `config apply` ohne `--dry-run` "nicht implementiert" zurückgibt, statt still nichts zu tun.** Der Live-Schreib-Endpunkt auf HYDRA-UMC-SERVER, den dies aufrufen würde, existiert tatsächlich noch nicht (dieselbe Lücke, an der auch `deploy`/`flash-all` hängen) - ein eigener `ExitNotImplemented`-Exit-Code sagt dem Aufrufer "das ist eine echte Lücke, kein Bug", statt eines irreführend erfolgreichen No-ops.
* **Warum die Fehler jedes Befehls jetzt über einen einzigen `CliError`/`ExitCode`-Typ laufen statt über Ad-hoc-Aufrufe von `os.Exit`.** Ein stabiler, dokumentierter Exit-Code-Vertrag bleibt nur stabil, wenn es eine einzige Stelle gibt, die Codes vergibt - `exitCodeFor()` (`exitcode.go`) ist diese Stelle, und die Befehlsfunktionen geben weiterhin idiomatische, verkettbare `error`-Werte zurück, statt selbst `os.Exit` aufzurufen.

---

## 📂 VERZEICHNISSTRUKTUR

Reiner Software-Dienst (CLI) — ohne eigene Hardware, Firmware oder Betriebssystem; diese Ordner werden gemäß der Repository-Strukturpolitik ausgelassen.

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Go-Modul
│   ├── go.mod                 # Modul-Definition (github.com/JuanenRac/HYDRA-UMC-TOOL-CLI)
│   └── cmd/hydra-cli/         # Binary-Einstiegspunkt
│       ├── main.go            # Befehls-Dispatch (version/help/status/robots/doctor/config)
│       ├── server.go          # Gemeinsame Auflösung von --server/HYDRA_CLI_SERVER
│       ├── robots.go          # Echter GET /api/settings-Client + Listenausgabe
│       ├── doctor.go          # Schreibgeschützte Zwei-Endpunkt-Vertragsdiagnose
│       ├── config.go          # Echtes Laden, Validieren und apply --dry-run der Konfigurationsdatei
│       ├── exitcode.go        # Echter, stabiler ExitCode/CliError-Vertrag
│       ├── *_test.go          # Echte Tests (net/http/httptest-Roundtrips, Fixtures mit temporären Dateien)
│       └── version.go         # const Version = "0.0.0"
├── docs/                      # Dokumentation: CLI_REFERENCE.md und DOCTOR.md
├── build/                     # Kompilierte Binärdateien (von git ignoriert)
├── images/                    # Medien und Diagramme
├── scripts/                   # Hilfsskripte
├── bump_version.py            # Versionserhöhung nach Kilometerzähler-Prinzip (vom Build ausgeführt)
├── build.sh / build.bat       # Echter Build: Erhöhung + echte Testsuite + go build + Rauchtest
├── run.sh / run.bat           # Echte Ausführung: startet die kompilierte Binärdatei
└── README.md
```

---

## 4. ⚙️ BUILD & AUSFÜHRUNG

Erfordert Go >= 1.21.

```bash
# Linux/macOS
./build.sh
./run.sh version
./run.sh status --server http://localhost:3000
./run.sh robots --server http://localhost:3000
./run.sh doctor --server http://localhost:3000
./run.sh config validate --config ./hydra-cli.json
./run.sh config apply --config ./hydra-cli.json --dry-run
echo $?   # 0=ok 2=Nutzung 3=Konfiguration 4=Netzwerk 5=Server 6=nicht-implementiert

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
run.bat doctor --server http://localhost:3000
run.bat config validate --config .\hydra-cli.json
run.bat config apply --config .\hydra-cli.json --dry-run
```

`build` erhöht die Version (`src/cmd/hydra-cli/version.go`), führt die echte Testsuite aus (`go vet` + `go test`), kompiliert das Go-Modul in `src/` nach `build/hydra-cli(.exe)` und führt einmalig `version` zur Verifikation aus. `run` führt die kompilierte Binärdatei erneut aus und leitet alle Argumente weiter — probiere `run doctor` gegen eine laufende `HYDRA-UMC-SERVER`-Instanz. Doctor ist eine sichere, schreibgeschützte Endpunkt-Vertragsprüfung; siehe [docs/DOCTOR.md](docs/DOCTOR.md).

---

## 🔗 Verwandte Projekte

Dieses Projekt ist Teil eines größeren Robotik-Ökosystems desselben Autors (JuanenRac / Electro Hobby 3D), das Firmware, Steuerungssoftware, KI-Knoten und Flotten-Tools umfasst. Gut zu wissen, denn eine Anfrage könnte tatsächlich eines dieser Projekte betreffen statt dieses Repository.

### Direkte Beziehung

- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — das Backend, das diese CLI im Flottenmaßstab verwaltet.
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — macht im Flottenmaßstab, was dieses Tool für eine einzelne Platine macht.
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — macht im Flottenmaßstab, was dieses Tool für eine einzelne Platine macht.

### Restliches Ökosystem

**HYDRA-UMC-Plattform** — die Multi-Roboter-Mikrofabrikzelle
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** — das CM5 + STM32H745-Motherboard, das bis zu 8 Roboterarme orchestriert.
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — das Express/WebSocket-Backend, mit dem jeder Steuerungsclient spricht.
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** — webbasiertes Steuerungs-Dashboard, Multi-Roboter-3D-Visualisierung.
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** — Android-Steuerungs-App über Wi-Fi/Bluetooth.
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** — iOS/iPadOS-Steuerungs-App, gebaut in Flutter.
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** — Desktop-Schwarm-Kommandozentrale (Python/PySide6).
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** — Desktop-URDF-Modelleditor für den Roboterkatalog.
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** — native Touch-UI für den eingebauten DSI-Touchscreen.

**URTC-Plattform** — der Werkzeugkopf-Controller, den jeder HYDRA-UMC-Roboterarm trägt
- **[URTC](https://github.com/JuanenRac/URTC)** — CAN-Bus-Werkzeugkopf-Controller, 25 Werkzeugprofile.
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — Desktop-Tool für CAN-OTA + SWD/JTAG-Flashing.
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — Desktop-Tool für Live-CAN-Bus-Diagnose.
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** — browserbasierte Alternative über die Web-Serial-API.

**🎥 Vision AI Node (Hailo-8)**
- [HYDRA-UMC-VISION-NODE](https://github.com/JuanenRac/HYDRA-UMC-VISION-NODE)
- [HYDRA-UMC-VISION-STREAMER](https://github.com/JuanenRac/HYDRA-UMC-VISION-STREAMER)
- [HYDRA-UMC-DETECTION-HEF](https://github.com/JuanenRac/HYDRA-UMC-DETECTION-HEF)
- [HYDRA-UMC-SAFETY-ZONES](https://github.com/JuanenRac/HYDRA-UMC-SAFETY-ZONES)
- [HYDRA-UMC-VISUAL-SERVOING-API](https://github.com/JuanenRac/HYDRA-UMC-VISUAL-SERVOING-API)

**🧠 Cognitive AI Node (Hailo-10)**
- [HYDRA-UMC-COGNITIVE-NODE](https://github.com/JuanenRac/HYDRA-UMC-COGNITIVE-NODE)
- [HYDRA-UMC-VLA-ENGINE](https://github.com/JuanenRac/HYDRA-UMC-VLA-ENGINE)
- [HYDRA-UMC-VOICE-UI](https://github.com/JuanenRac/HYDRA-UMC-VOICE-UI)
- [HYDRA-UMC-SEMANTIC-PLANNER](https://github.com/JuanenRac/HYDRA-UMC-SEMANTIC-PLANNER)
- [HYDRA-UMC-DOCS-QA](https://github.com/JuanenRac/HYDRA-UMC-DOCS-QA)

**🐝 Orchestration & Swarm**
- [HYDRA-UMC-ORCHESTRATOR](https://github.com/JuanenRac/HYDRA-UMC-ORCHESTRATOR)
- [HYDRA-UMC-SWARM-SYNC](https://github.com/JuanenRac/HYDRA-UMC-SWARM-SYNC)
- [HYDRA-UMC-PATH-PLANNER-3D](https://github.com/JuanenRac/HYDRA-UMC-PATH-PLANNER-3D)
- [HYDRA-UMC-JOB-DISPATCHER](https://github.com/JuanenRac/HYDRA-UMC-JOB-DISPATCHER)
- [HYDRA-UMC-NODE-HEALING](https://github.com/JuanenRac/HYDRA-UMC-NODE-HEALING)

**🎮 Digital Twin & Simulation**
- [HYDRA-UMC-TWIN](https://github.com/JuanenRac/HYDRA-UMC-TWIN)
- [HYDRA-UMC-PHYSICS-REPLICA](https://github.com/JuanenRac/HYDRA-UMC-PHYSICS-REPLICA)
- [HYDRA-UMC-HIL-BRIDGE](https://github.com/JuanenRac/HYDRA-UMC-HIL-BRIDGE)
- [HYDRA-UMC-SYNTHETIC-DATA-GEN](https://github.com/JuanenRac/HYDRA-UMC-SYNTHETIC-DATA-GEN)

**📊 Data & Analytics**
- [HYDRA-UMC-DATALAKE](https://github.com/JuanenRac/HYDRA-UMC-DATALAKE)
- [HYDRA-UMC-TELEMETRY-COLLECTOR](https://github.com/JuanenRac/HYDRA-UMC-TELEMETRY-COLLECTOR)
- [HYDRA-UMC-ANOMALY-DETECTOR](https://github.com/JuanenRac/HYDRA-UMC-ANOMALY-DETECTOR)
- [HYDRA-UMC-PRODUCTION-REPORTS](https://github.com/JuanenRac/HYDRA-UMC-PRODUCTION-REPORTS)

**🏭 Industrial Gateway**
- [HYDRA-UMC-GATEWAY-INDUSTRIAL](https://github.com/JuanenRac/HYDRA-UMC-GATEWAY-INDUSTRIAL)
- [HYDRA-UMC-OPCUA-SERVER](https://github.com/JuanenRac/HYDRA-UMC-OPCUA-SERVER)
- [HYDRA-UMC-MQTT-BROKER](https://github.com/JuanenRac/HYDRA-UMC-MQTT-BROKER)
- [HYDRA-UMC-MTCONNECT-ADAPTER](https://github.com/JuanenRac/HYDRA-UMC-MTCONNECT-ADAPTER)

**🛠️ Complementary Tools**
- [URTC-SMART-RACK](https://github.com/JuanenRac/URTC-SMART-RACK)
- [URTC-VISION-TOOL](https://github.com/JuanenRac/URTC-VISION-TOOL)
- [HYDRA-UMC-WATCH](https://github.com/JuanenRac/HYDRA-UMC-WATCH)
- [HYDRA-UMC-DASHBOARD-AI](https://github.com/JuanenRac/HYDRA-UMC-DASHBOARD-AI)


## 👤 AUTOR
**JuanenRac** (Electro Hobby 3D)
📧 electrohobby3d@gmail.com

## 📜 LIZENZ
GPL-3.0 - Siehe LICENSE für Details.

## 🛠️ BUILD & RUN

Verwenden Sie den Build-Check ohne Versionierung vor einem Release-Build:

| Aktion | Windows | Linux / macOS |
|---|---|---|
| Build-Check (ohne Änderung von Version oder CHANGELOG) | `build-test.bat` | `./build-test.sh` |
| Ausführung / Entwicklung (falls vorhanden) | `run*.bat` oder `dev*.bat` | `./run*.sh` oder `./dev*.sh` |

`build-test.bat` und `build-test.sh` kompilieren oder validieren den Projekt-Stack, ohne `hydra-umc.project.json` zu erhöhen oder `CHANGELOG.md` zu verändern. Sie dürfen nur normale Compiler-Ausgaben erzeugen. Die vorhandenen Skripte `build*.bat`, `build*.sh`, `run*` und `dev*` behalten ihr projektbezogenes Versions- oder Laufzeitverhalten bei; verwenden Sie sie, wenn dieses Verhalten benötigt wird.
