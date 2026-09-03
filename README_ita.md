<p align="center">
  <img src="images/HYDRA_UMC_BANNER.svg" alt="HYDRA-UMC-TOOL-CLI banner" width="100%">
</p>

# 💻 HYDRA-UMC-TOOL-CLI

<p align="center"><a href="README.md">🇺🇸 English</a> | <a href="README_spa.md">🇪🇸 Español</a> | <a href="README_fra.md">🇫🇷 Français</a> | 🇮🇹 <b>Italiano</b> | <a href="README_deu.md">🇩🇪 Deutsch</a> | <a href="README_zho.md">🇨🇳 简体中文</a> | <a href="README_jpn.md">🇯🇵 日本語</a></p>

### 🛠️ Interfaccia a Riga di Comando per DevOps e Automazione della Flotta

<p align="left">
  <img src="https://img.shields.io/badge/Licencia-GPL%203.0-blue.svg" alt="GPL 3.0">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Feature-Fleet%20DevOps-blue.svg" alt="DevOps">
</p>

---

## 1. 🛠️ PANORAMICA TECNICA

**HYDRA-UMC-TOOL-CLI** è il coltellino svizzero per sviluppatori e amministratori di sistema dell'ecosistema HYDRA-UMC. È un singolo binario statico Go che fornisce strumenti a riga di comando per interrogare, aggiornare e verificare i deployment di HYDRA-UMC.

L'obiettivo a lungo termine sono deployment massivi di missioni, aggiornamenti firmware in parallelo (CAN-OTA) e diagnostiche di sistema approfondite direttamente da un terminale o da una pipeline CI/CD. Oggi offre la base reale e funzionante su cui si costruirà tutto il resto: la segnalazione della versione e un client HTTP reale verso HYDRA-UMC-SERVER.

### Caratteristiche Principali:
* ✅ **`hydra-cli version`** — stampa nome e versione del CLI stesso. *(implementato)*
* ✅ **`hydra-cli status [--server URL]`** — interroga il `GET /api/hydra-info` di un'istanza attiva di HYDRA-UMC-SERVER e ne stampa l'identità dichiarata. *(implementato)*
* ✅ **`hydra-cli robots [--server URL]`** — interroga il `GET /api/settings` di un'istanza attiva di HYDRA-UMC-SERVER e ne stampa l'elenco reale di controller/robot (nome, stato online, modello, ruolo). *(implementato)*
* ✅ **`hydra-cli doctor [--server URL]`** — diagnostica del contratto del server in sola lettura: convalida `/api/hydra-info` e `/api/settings` e confronta i totali pubblicati di controller/robot con l'elenco. Non invia comandi né sonda hardware. *(implementato)*
* ✅ **Un contratto di exit code reale e stabile** — `0` ok, `1` errore generale, `2` errore d'uso, `3` errore di configurazione, `4` errore di rete, `5` errore del server, `6` non implementato. Ogni comando classifica i propri fallimenti tramite questo contratto invece di un semplice `exit 1`, così gli script che avvolgono questa CLI possono ramificarsi in base al *perché* è fallito. *(implementato)*
* ✅ **`hydra-cli config validate --config PATH`** — carica e valida secondo schema un file di configurazione locale (URL del server, timeout delle richieste). *(implementato)*
* ✅ **`hydra-cli config apply --config PATH [--dry-run]`** — `--dry-run` dimostra il vero percorso di validazione end-to-end e stampa esattamente cosa invierebbe; senza di esso, restituisce onestamente "non implementato" poiché non esiste ancora un endpoint di scrittura della flotta live. *(implementato, solo dry-run)*
* ✅ **`hydra-cli shell [--server URL]`** — una REPL interattiva: esegue ripetutamente uno qualsiasi dei comandi sopra contro lo stesso server senza riavviare il processo. Instrada attraverso la stessa tabella dei comandi usata dalle invocazioni singole, quindi il comportamento della shell e quello singolo non possono mai divergere. `exit`/`quit`/Ctrl-D per uscire. *(implementato)*
* ✅ **`hydra-cli help` / `--help`** — utilizzo completo dei comandi. *(implementato)*
* 🚧 **`hydra-cli deploy`** — carica missioni e configurazioni su una flotta di robot simultaneamente. *(pianificato)*
* 🚧 **`hydra-cli flash-all`** — aggiornamenti firmware in parallelo per controller e teste URTC. *(pianificato)*
* 🚧 **`hydra-cli audit`** — suite diagnostica automatizzata per la salute del bus CAN e la validazione dei sensori. *(pianificato)*

---

## 2. 🔄 FLUSSO DEL CLI

```mermaid
flowchart LR
    USER["Sviluppatore / DevOps"] --> CLI["HYDRA-UMC-TOOL-CLI"]
    CLI -- HTTP --> SERVER["HYDRA-UMC-SERVER (/api/hydra-info)"]
    SERVER -- Stato della Flotta --> CLI
    CLI -- Risultato --> USER
```

---

## 3. 🧱 ARCHITETTURA E DECISIONI DI PROGETTAZIONE

* **Perché `src/` contiene un sotto-percorso `cmd/hydra-cli/`, non un layout piatto.** Segue la convenzione standard Go per le CLI (un entry point `cmd/<nome-binario>/`, con spazio per futuri pacchetti `internal/`/`pkg/` man mano che la CLI cresce oltre un singolo comando) - non un'invenzione di questo ecosistema, la convenzione propria della più ampia comunità Go per le CLI multi-comando.
* **Perché una CLI, e non semplicemente scriptare direttamente l'API REST propria di HYDRA-UMC-SERVER.** Le operazioni su scala flotta (installare/aggiornare su molti CM5, non solo uno) richiedono una vera orchestrazione - retry, parallelismo, una UX coerente - che uno script curl estemporaneo non offre, lo stesso motivo che HYDRA-UMC-UPDATER applica più avanti a livello dell'intero checkout dell'ecosistema.
* **Come si inserisce nel resto dell'ecosistema.** Fa su scala flotta ciò che URTC-FLASHER e URTC-TESTER fanno ciascuno per una singola scheda - gestisce istanze di HYDRA-UMC-SERVER attraverso una flotta invece del firmware proprio di una singola scheda.
* **Perché `robots` legge `GET /api/settings` invece di un nuovo endpoint.** Quell'endpoint porta già l'elenco completo di controller/robot ed è già una lettura reale, non autenticata (vedi il proprio `src/server.ts` di HYDRA-UMC-SERVER) - `robots` è un vero client di un contratto già pubblicato, non nuovo lavoro lato server. `doctor` usa questa stessa lettura con `/api/hydra-info` per rilevare totali pubblici incompatibili senza aggiungere un endpoint. I comandi più grandi, ancora pianificati (`deploy`/`flash-all`/`audit` orientato all'hardware), necessitano davvero di nuovi endpoint di scrittura che non esistono ancora.
* **Perché `doctor` è esplicitamente in sola lettura.** Un controllo del contratto è utile prima di disporre dell'hardware e sicuro in CI. Riporta solo coerenza HTTP/JSON/contatori; non comanda apparecchiature né dichiara salute di CAN, attuatori, sensori, camere, Hailo, CM5 o sicurezza.
* **Perché `config apply` senza `--dry-run` restituisce "non implementato" invece di non fare nulla silenziosamente.** L'endpoint di scrittura live su HYDRA-UMC-SERVER che verrebbe chiamato in questo caso davvero non esiste ancora (lo stesso gap su cui sono bloccati `deploy`/`flash-all`) - un exit code distinto, `ExitNotImplemented`, dice a chi chiama "questo è un gap reale, non un bug", invece di un no-op ingannevolmente riuscito.
* **Perché gli errori di ogni comando ora passano attraverso un unico tipo `CliError`/`ExitCode` invece di chiamate ad-hoc a `os.Exit`.** Un contratto di exit code stabile e documentato resta tale solo se esiste un unico punto che assegna i codici - `exitCodeFor()` (`exitcode.go`) è quel punto, e le funzioni dei comandi continuano a restituire valori `error` idiomatici e concatenabili invece di chiamare `os.Exit` da sole.

---

## 📂 STRUTTURA DELLE DIRECTORY

Servizio puramente software (CLI) — senza hardware, firmware o sistema operativo propri; tali cartelle sono omesse secondo la politica della struttura del repository.

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Modulo Go
│   ├── go.mod                 # Definizione del modulo (github.com/JuanenRac/HYDRA-UMC-TOOL-CLI)
│   └── cmd/hydra-cli/         # Punto di ingresso del binario
│       ├── main.go            # Smistamento dei comandi (version/help/status/robots/doctor/config)
│       ├── server.go          # Risoluzione condivisa di --server/HYDRA_CLI_SERVER
│       ├── robots.go          # Vero client GET /api/settings + stampa dell'elenco
│       ├── doctor.go          # Diagnostica di contratto a due endpoint, sola lettura
│       ├── config.go          # Caricamento reale del file di configurazione, validazione, apply --dry-run
│       ├── exitcode.go        # Contratto reale e stabile ExitCode/CliError
│       ├── *_test.go          # Test reali (round-trip net/http/httptest, fixture di file temporanei)
│       └── version.go         # const Version - incremento contachilometri, sincronizzato con il manifesto
├── docs/                      # Documentazione: CLI_REFERENCE.md e DOCTOR.md
├── build/                     # Binari compilati (ignorato da git)
├── images/                    # Media e diagrammi
├── bump_version.py            # Incremento versione nativa stile contachilometri (eseguito dal build)
├── bump_manifest_version.py   # Sincronizza la versione di hydra-umc.project.json con quella nativa (--sync)
├── build.sh / build.bat       # Build reale: incremento + vera suite di test + go build + smoke test
├── run.sh / run.bat           # Esecuzione reale: avvia il binario compilato
└── README.md
```

---

## 4. ⚙️ COMPILAZIONE ED ESECUZIONE

Richiede Go >= 1.21.

```bash
# Linux/macOS
./build.sh
./run.sh version
./run.sh status --server http://localhost:3000
./run.sh robots --server http://localhost:3000
./run.sh doctor --server http://localhost:3000
./run.sh config validate --config ./hydra-cli.json
./run.sh config apply --config ./hydra-cli.json --dry-run
echo $?   # 0=ok 2=uso 3=config 4=rete 5=server 6=non-implementato

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
run.bat doctor --server http://localhost:3000
run.bat config validate --config .\hydra-cli.json
run.bat config apply --config .\hydra-cli.json --dry-run
```

`build` incrementa la versione (`src/cmd/hydra-cli/version.go`), esegue la vera suite di test (`go vet` + `go test`), compila il modulo Go in `src/` in `build/hydra-cli(.exe)` ed esegue `version` una volta per verificare. `run` riesegue il binario compilato, inoltrando tutti gli argomenti — prova `run doctor` contro un'istanza `HYDRA-UMC-SERVER` in esecuzione. Doctor è un controllo sicuro e in sola lettura dei contratti endpoint; vedi [docs/DOCTOR.md](docs/DOCTOR.md).

---

## 🔗 Progetti Correlati

Questo progetto fa parte dell'ecosistema robotico HYDRA-UMC dello stesso autore (JuanenRac / Electro Hobby 3D). Vale la pena conoscerlo, poiché una richiesta potrebbe in realtà riguardare uno di questi invece di questo repository.

**Direttamente Correlati**
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — il vero backend headless (REST/WebSocket) con cui parla davvero ogni client di controllo — il backend che questa CLI gestisce su scala di flotta.
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — strumento desktop con GUI per il flashing delle schede URTC, CAN-OTA più SWD/JTAG a chip intero — il deploy CAN-OTA a scala di flotta pianificato di questo strumento fa per molte schede ciò che URTC-FLASHER fa per una sola.
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — strumento desktop di diagnostica CAN-bus dal vivo per schede URTC, un pannello per profilo utensile — la diagnostica a scala di flotta pianificata di questo strumento fa per molte schede ciò che URTC-TESTER fa per una sola.

**Fa Anche Parte dell'Ecosistema**

*Hardware e Piattaforma di Base*
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** — la scheda madre fisica del braccio robotico: host CM5 + coprocessore STM32H745 dual-core, che coordina fino a 8 bracci utensile via CAN-OTA/SPI-OTA.
- **[HYDRA-UMC-OS](https://github.com/JuanenRac/HYDRA-UMC-OS)** — livello prodotto riproducibile su Raspberry Pi OS per il CM5: agente in sola lettura, config/profili validati, provisioning WiFi al primo contatto.
- **[HYDRA-UMC-SDK](https://github.com/JuanenRac/HYDRA-UMC-SDK)** — il contratto JSON-Schema condiviso e la barriera di sicurezza contro cui ogni bridge valida i propri comandi.

*Backend Centrale e Client*
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** — dashboard di controllo web con visualizzazione 3D multi-robot in tempo reale.
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** — centro di comando sciame desktop (PySide6) per più server contemporaneamente, pacchettizzato come eseguibile standalone.
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** — app di controllo nativa per Android con login biometrico e un companion Wear OS abbinato.
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** — app di controllo per iOS/iPadOS (Flutter) con sincronizzazione WebSocket in tempo reale.
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** — interfaccia touch nativa per il touchscreen DSI da 7" a bordo, incorporata direttamente nel CM5.
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** — creatore/editor grafico desktop di URDF che invia i modelli finiti al catalogo di STUDIO.
- **[HYDRA-UMC-BRIDGE-AMR](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-AMR)** — barriera di coordinamento per flotte AGV/AMR tramite un publisher MQTT VDA 5050 reale.
- **[HYDRA-UMC-BRIDGE-CNC](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-CNC)** — coordinatore ad alto livello per celle CNC con accesso reale a stato/byte di controllo GRBL.
- **[HYDRA-UMC-BRIDGE-DROIDS](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-DROIDS)** — barriera di coordinamento per droidi con zampe/umanoidi, con un vero mittente di comandi per Boston Dynamics Spot.
- **[HYDRA-UMC-BRIDGE-LASER](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-LASER)** — coordinatore di sicurezza per celle laser che legge 3 salvaguardie GPIO reali di chiave/involucro/interblocco.
- **[HYDRA-UMC-BRIDGE-OPENPNP](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-OPENPNP)** — coordinatore ad alto livello sicuro per il flusso schede del pick-and-place OpenPnP.
- **[HYDRA-UMC-BRIDGE-PRINTER3D](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-PRINTER3D)** — barriera di coordinamento sicura per stampanti 3D Moonraker/Klipper, con comandi di lavoro reali e controllati.
- **[HYDRA-UMC-BRIDGE-ROS2](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-ROS2)** — coordinatore di sicurezza con un vero trasporto ROS 2 rclpy, importato in modo lazy.
- **[HYDRA-UMC-BRIDGE-UAV](https://github.com/JuanenRac/HYDRA-UMC-BRIDGE-UAV)** — barriera di coordinamento per UAV dotati di fotocamera, con un vero mittente di comandi MAVLink.

*Piattaforma Strumenti URTC*
- **[URTC](https://github.com/JuanenRac/URTC)** — firmware per la scheda fisica dell'Universal Robot Tool Controller, oltre 25 profili utensile su bus CAN.
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** — alternativa basata su browser a URTC-TESTER tramite la Web Serial API, senza installazione locale.

*Nodo IA Visione (Hailo-8)*
- **[HYDRA-UMC-VISION-NODE](https://github.com/JuanenRac/HYDRA-UMC-VISION-NODE)** — hub di integrazione per la pipeline di visione Hailo-8, con un vero controllo di prontezza hardware per fase.
- **[HYDRA-UMC-DETECTION-HEF](https://github.com/JuanenRac/HYDRA-UMC-DETECTION-HEF)** — registro reale di modelli compilati con verifica di caricamento sicuro per architettura Hailo/checksum.
- **[HYDRA-UMC-VISION-STREAMER](https://github.com/JuanenRac/HYDRA-UMC-VISION-STREAMER)** — generatore reale di pipeline GStreamer + config MediaMTX, con una vera barriera di integrazione HailoRT.
- **[HYDRA-UMC-VISUAL-SERVOING-API](https://github.com/JuanenRac/HYDRA-UMC-VISUAL-SERVOING-API)** — vera legge di correzione Position-Based Visual Servoing, con cancello di sicurezza sullo stato di zona a monte.
- **[HYDRA-UMC-SAFETY-ZONES](https://github.com/JuanenRac/HYDRA-UMC-SAFETY-ZONES)** — vero controllo di violazione zona e richiesta E-STOP, con imposizione della freschezza di calibrazione.

*Nodo IA Cognitivo (Hailo-10)*
- **[HYDRA-UMC-COGNITIVE-NODE](https://github.com/JuanenRac/HYDRA-UMC-COGNITIVE-NODE)** — hub di integrazione per la pipeline cognitiva Hailo-10 (orchestrazione LLM/VLA/voce).
- **[HYDRA-UMC-VLA-ENGINE](https://github.com/JuanenRac/HYDRA-UMC-VLA-ENGINE)** — vera codifica/decodifica di token d'azione e generazione di traiettoria per un modello Vision-Language-Action.
- **[HYDRA-UMC-VOICE-UI](https://github.com/JuanenRac/HYDRA-UMC-VOICE-UI)** — vero front-end vocale (VAD + parser di intenti) con un relay verso Watch limitato e soggetto a conferma.
- **[HYDRA-UMC-SEMANTIC-PLANNER](https://github.com/JuanenRac/HYDRA-UMC-SEMANTIC-PLANNER)** — vera scomposizione dei task basata su regole e recupero semantico degli errori sui codici errore MCU.
- **[HYDRA-UMC-DOCS-QA](https://github.com/JuanenRac/HYDRA-UMC-DOCS-QA)** — vera ricerca documentale TF-IDF (solo libreria standard) sui documenti Markdown di questo ecosistema.

*Orchestrazione e Sciame*
- **[HYDRA-UMC-ORCHESTRATOR](https://github.com/JuanenRac/HYDRA-UMC-ORCHESTRATOR)** — hub di integrazione con un vero contratto di health-report gRPC/Protobuf e una macchina a stati di missione.
- **[HYDRA-UMC-JOB-DISPATCHER](https://github.com/JuanenRac/HYDRA-UMC-JOB-DISPATCHER)** — vera coda di lavori basata su priorità con deduplicazione, su una vera API HTTP.
- **[HYDRA-UMC-NODE-HEALING](https://github.com/JuanenRac/HYDRA-UMC-NODE-HEALING)** — vero watchdog di salute della flotta basato su gRPC, con retry/backoff e rilevamento di discrepanza d'identità.
- **[HYDRA-UMC-PATH-PLANNER-3D](https://github.com/JuanenRac/HYDRA-UMC-PATH-PLANNER-3D)** — vero pianificatore di percorsi 3D basato su RRT, con vera validazione delle collisioni ostacolo/spazio di lavoro.
- **[HYDRA-UMC-SWARM-SYNC](https://github.com/JuanenRac/HYDRA-UMC-SWARM-SYNC)** — vera sincronizzazione di stato CRDT LWW-Element-Map, con property test per la convergenza multi-cella.

*Gemello Digitale e Simulazione*
- **[HYDRA-UMC-TWIN](https://github.com/JuanenRac/HYDRA-UMC-TWIN)** — hub di integrazione per il motore di gemello digitale, con un vero contratto di sincronizzazione per compatibilità di versione.
- **[HYDRA-UMC-HIL-BRIDGE](https://github.com/JuanenRac/HYDRA-UMC-HIL-BRIDGE)** — vero interblocco di sicurezza hardware-in-the-loop che instrada i comandi tra simulazione e hardware reale.
- **[HYDRA-UMC-PHYSICS-REPLICA](https://github.com/JuanenRac/HYDRA-UMC-PHYSICS-REPLICA)** — vera cinematica diretta e validazione dei limiti articolari su un vero sottoinsieme URDF.
- **[HYDRA-UMC-SYNTHETIC-DATA-GEN](https://github.com/JuanenRac/HYDRA-UMC-SYNTHETIC-DATA-GEN)** — vero generatore procedurale di scene 2D con esportazione di annotazioni YOLO/COCO.

*Dati e Analisi*
- **[HYDRA-UMC-DATALAKE](https://github.com/JuanenRac/HYDRA-UMC-DATALAKE)** — vero archivio di serie temporali basato su sqlite3, con una vera API HTTP di ingestione/query.
- **[HYDRA-UMC-ANOMALY-DETECTOR](https://github.com/JuanenRac/HYDRA-UMC-ANOMALY-DETECTOR)** — vero rilevatore di anomalie FFT + baseline statistica, con monitoraggio della deriva.
- **[HYDRA-UMC-PRODUCTION-REPORTS](https://github.com/JuanenRac/HYDRA-UMC-PRODUCTION-REPORTS)** — vero calcolo OEE/disponibilità sullo storico di DATALAKE, con esportazione CSV riproducibile.
- **[HYDRA-UMC-TELEMETRY-COLLECTOR](https://github.com/JuanenRac/HYDRA-UMC-TELEMETRY-COLLECTOR)** — vera pipeline di ingestione CAN/WebSocket verso DATALAKE, con deduplicazione per sequenza.

*Gateway Industriale*
- **[HYDRA-UMC-GATEWAY-INDUSTRIAL](https://github.com/JuanenRac/HYDRA-UMC-GATEWAY-INDUSTRIAL)** — hub di integrazione che inoltra ai protocolli industriali, con un vero livello di allowlist dei comandi/backpressure.
- **[HYDRA-UMC-OPCUA-SERVER](https://github.com/JuanenRac/HYDRA-UMC-OPCUA-SERVER)** — vero spazio di indirizzi OPC-UA, verificato con una vera sessione client del protocollo binario.
- **[HYDRA-UMC-MQTT-BROKER](https://github.com/JuanenRac/HYDRA-UMC-MQTT-BROKER)** — vero broker MQTT con autenticazione opzionale per client e ACL sui topic.
- **[HYDRA-UMC-MTCONNECT-ADAPTER](https://github.com/JuanenRac/HYDRA-UMC-MTCONNECT-ADAPTER)** — veri endpoint XML `/probe` e `/current` di MTConnect, con output in modalità degradata.

*Strumenti Complementari e Operazioni dell'Ecosistema*
- **[HYDRA-UMC-DASHBOARD-AI](https://github.com/JuanenRac/HYDRA-UMC-DASHBOARD-AI)** — pannelli Smart Summaries e Anomaly Highlighting su DATALAKE/ANOMALY-DETECTOR, con un fallback statistico onesto.
- **[HYDRA-UMC-WATCH](https://github.com/JuanenRac/HYDRA-UMC-WATCH)** — app companion WearOS con avvisi aptici reali e un relay vocale verso il telefono abbinato.
- **[URTC-SMART-RACK](https://github.com/JuanenRac/URTC-SMART-RACK)** — firmware per un rack di montaggio schede con decodifica reale dell'ID utensile e logica di preriscaldamento Smart Idle.
- **[URTC-VISION-TOOL](https://github.com/JuanenRac/URTC-VISION-TOOL)** — firmware più un vero companion di visione Python per una testa utensile di ispezione termica/RGB.
- **[HYDRA-UMC-UPDATER](https://github.com/JuanenRac/HYDRA-UMC-UPDATER)** — strumento amministrativo desktop che scopre, clona e aggiorna ogni repository di questo ecosistema.


---

## 📚 Documentazione e Comunità

- **[CONTRIBUTING.md](CONTRIBUTING.md)** — stack tecnologico e linee guida di codifica per una pull request.
- **[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)** — gli standard di comportamento attesi in questa comunità.
- **[SECURITY.md](SECURITY.md)** — come segnalare una vulnerabilità, e le reali aree di attenzione sulla sicurezza di questo progetto.
- **[SUPPORT.md](SUPPORT.md)** — dove porre domande e segnalare bug.
- **[LICENSE.md](LICENSE.md)** — la licenza propria di questo progetto.

## 👤 AUTORE
**JuanenRac** (Electro Hobby 3D)
📧 electrohobby3d@gmail.com
📺 [youtube.com/@electrohobby3d](https://youtube.com/@electrohobby3d)

## 📜 LICENZA
GPL-3.0 - Vedi LICENSE per i dettagli.
