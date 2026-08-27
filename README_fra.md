<p align="center">
  <img src="images/HYDRA_UMC_BANNER.svg" alt="HYDRA-UMC-TOOL-CLI banner" width="100%">
</p>

# 💻 HYDRA-UMC-TOOL-CLI

<p align="center"><a href="README.md">🇺🇸 English</a> | <a href="README_spa.md">🇪🇸 Español</a> | 🇫🇷 <b>Français</b> | <a href="README_ita.md">🇮🇹 Italiano</a> | <a href="README_deu.md">🇩🇪 Deutsch</a> | <a href="README_zho.md">🇨🇳 简体中文</a> | <a href="README_jpn.md">🇯🇵 日本語</a></p>

### 🛠️ Interface en Ligne de Commande pour le DevOps et l'Automatisation de Flottes

<p align="left">
  <img src="https://img.shields.io/badge/Licencia-GPL%203.0-blue.svg" alt="GPL 3.0">
  <img src="https://img.shields.io/badge/Language-Go-00ADD8.svg" alt="Go">
  <img src="https://img.shields.io/badge/Feature-Fleet%20DevOps-blue.svg" alt="DevOps">
</p>

---

## 1. 🛠️ APERÇU TECHNIQUE

**HYDRA-UMC-TOOL-CLI** est le couteau suisse des développeurs et administrateurs système de l'écosystème HYDRA-UMC. Il s'agit d'un binaire Go statique unique fournissant des outils en ligne de commande pour interroger, mettre à jour et auditer les déploiements HYDRA-UMC.

L'objectif à long terme est de permettre des déploiements massifs de missions, des mises à jour de firmware en parallèle (CAN-OTA) et des diagnostics système approfondis directement depuis un terminal ou un pipeline CI/CD. Aujourd'hui, il fournit la base réelle et fonctionnelle sur laquelle tout le reste s'appuiera : le rapport de version et un client HTTP réel vers HYDRA-UMC-SERVER.

### Fonctionnalités Clés :
* ✅ **`hydra-cli version`** — affiche le nom et la version du CLI. *(implémenté)*
* ✅ **`hydra-cli status [--server URL]`** — interroge le `GET /api/hydra-info` d'une instance HYDRA-UMC-SERVER active et affiche son identité déclarée. *(implémenté)*
* ✅ **`hydra-cli robots [--server URL]`** — interroge le `GET /api/settings` d'une instance HYDRA-UMC-SERVER active et affiche sa liste réelle de contrôleurs/robots (nom, statut en ligne, modèle, rôle). *(implémenté)*
* ✅ **`hydra-cli help` / `--help`** — usage complet des commandes. *(implémenté)*
* 🚧 **`hydra-cli deploy`** — envoie missions et configurations vers une flotte de robots simultanément. *(prévu)*
* 🚧 **`hydra-cli flash-all`** — mises à jour de firmware en parallèle pour contrôleurs et têtes URTC. *(prévu)*
* 🚧 **`hydra-cli audit`** — suite de diagnostic automatisée pour la santé du bus CAN et la validation des capteurs. *(prévu)*

---

## 2. 🔄 FLUX DU CLI

```mermaid
flowchart LR
    USER["Développeur / DevOps"] --> CLI["HYDRA-UMC-TOOL-CLI"]
    CLI -- HTTP --> SERVER["HYDRA-UMC-SERVER (/api/hydra-info)"]
    SERVER -- État de la Flotte --> CLI
    CLI -- Résultat --> USER
```

---

## 3. 🧱 ARCHITECTURE & DÉCISIONS DE CONCEPTION

* **Pourquoi `src/` contient un sous-chemin `cmd/hydra-cli/`, pas une disposition plate.** Suit la convention Go standard pour les CLI (un point d'entrée `cmd/<nom-binaire>/`, avec de la place pour de futurs paquets `internal/`/`pkg/` à mesure que la CLI dépasse une seule commande) - pas une invention de cet écosystème, la propre convention de la communauté Go au sens large pour les CLI multi-commandes.
* **Pourquoi une CLI, plutôt que simplement scripter directement la propre API REST de HYDRA-UMC-SERVER.** Les opérations à l'échelle de la flotte (installer/mettre à jour sur de nombreux CM5, pas un seul) nécessitent une vraie orchestration - retries, parallélisme, une UX cohérente - qu'un script curl ponctuel n'offre pas, la même raison que HYDRA-UMC-UPDATER applique plus tard au niveau de tout le checkout de l'écosystème.
* **Pourquoi le point d'entrée ne fait qu'imprimer identité/version/rôle aujourd'hui.** Étape d'andamiaje : prouver que `go build ./cmd/hydra-cli` réussit précède le vrai jeu de commandes de gestion de flotte.
* **Comment cela s'intègre dans le reste de l'écosystème.** Fait à l'échelle de la flotte ce qu'URTC-FLASHER et URTC-TESTER font chacun pour une seule carte - gère des instances de HYDRA-UMC-SERVER à travers une flotte plutôt que le propre firmware d'une seule carte.
* **Pourquoi `robots` lit `GET /api/settings` plutôt qu'un nouvel endpoint.** Cet endpoint porte déjà la liste complète des contrôleurs/robots et est déjà une vraie lecture réelle, non authentifiée (voir le propre `src/server.ts` de HYDRA-UMC-SERVER) - `robots` est un vrai client d'un contrat déjà livré, pas un nouveau travail côté serveur. Les commandes plus importantes, encore prévues (`deploy`/`flash-all`/`audit`), ont réellement besoin de nouveaux endpoints d'écriture qui n'existent pas encore.

---

## 📂 STRUCTURE DES DOSSIERS

Service purement logiciel (CLI) — sans hardware/firmware/os propres, élagués du modèle (voir `SONNET/5.PLAN_EJECUCION_32_PROYECTOS_NUEVOS.txt` pour la règle d'élagage de tout l'écosystème).

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Module Go
│   ├── go.mod                 # Définition du module (github.com/JuanenRac/HYDRA-UMC-TOOL-CLI)
│   └── cmd/hydra-cli/         # Point d'entrée du binaire
│       ├── main.go            # Répartition des commandes (version/help/status/robots)
│       ├── server.go          # Résolution partagée de --server/HYDRA_CLI_SERVER
│       ├── robots.go          # Vrai client GET /api/settings + affichage de la liste
│       ├── *_test.go          # Vrais tests (allers-retours net/http/httptest)
│       └── version.go         # const Version = "0.0.0"
├── docs/                      # Documentation et référence des commandes
├── build/                     # Binaires compilés (ignoré par git)
├── images/                    # Médias et diagrammes
├── scripts/                   # Scripts utilitaires
├── bump_version.py            # Incrémentation de version façon compteur kilométrique (exécuté par build)
├── build.sh / build.bat       # Build réel : incrémentation + vraie suite de tests + go build + test de fumée
├── run.sh / run.bat           # Exécution réelle : lance le binaire compilé
└── README.md
```

---

## 4. ⚙️ COMPILATION ET EXÉCUTION

Nécessite Go >= 1.21.

```bash
# Linux/macOS
./build.sh
./run.sh version
./run.sh status --server http://localhost:3000
./run.sh robots --server http://localhost:3000

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
```

`build` incrémente la version (`src/cmd/hydra-cli/version.go`), exécute la vraie suite de tests (`go vet` + `go test`), compile le module Go dans `src/` vers `build/hydra-cli(.exe)`, puis exécute `version` une fois pour vérifier. `run` relance le binaire compilé en transmettant tous les arguments — essayez `run status` ou `run robots` face à une instance `HYDRA-UMC-SERVER` en cours d'exécution.

---

## 🔗 Projets Liés

Ce projet fait partie d'un écosystème robotique plus large du même auteur (JuanenRac / Electro Hobby 3D), couvrant firmware, logiciel de contrôle, nœuds IA et outillage de flotte. Bon à savoir, car une demande pourrait en réalité concerner l'un de ces projets plutôt que ce dépôt.

### Relation Directe

- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — le backend que gère cette CLI à l'échelle de la flotte.
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — fait à l'échelle de la flotte ce que cet outil fait pour une carte.
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — fait à l'échelle de la flotte ce que cet outil fait pour une carte.

### Reste de l'Écosystème

**Plateforme HYDRA-UMC** — la cellule de micro-usine multi-robot
- **[HYDRA-UMC](https://github.com/JuanenRac/HYDRA-UMC)** — la carte mère CM5 + STM32H745 orchestrant jusqu'à 8 bras robotiques.
- **[HYDRA-UMC-SERVER](https://github.com/JuanenRac/HYDRA-UMC-SERVER)** — le backend Express/WebSocket auquel parle chaque client de contrôle.
- **[HYDRA-UMC-STUDIO](https://github.com/JuanenRac/HYDRA-UMC-STUDIO)** — tableau de bord de contrôle web, visualisation 3D multi-robot.
- **[HYDRA-UMC-ANDROID-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-ANDROID-CONTROL)** — application de contrôle Android via Wi-Fi/Bluetooth.
- **[HYDRA-UMC-IOS-CONTROL](https://github.com/JuanenRac/HYDRA-UMC-IOS-CONTROL)** — application de contrôle iOS/iPadOS construite en Flutter.
- **[HYDRA-UMC-SUITE](https://github.com/JuanenRac/HYDRA-UMC-SUITE)** — centre de commande d'essaim de bureau (Python/PySide6).
- **[HYDRA-UMC-EDITOR-URDF](https://github.com/JuanenRac/HYDRA-UMC-EDITOR-URDF)** — éditeur de modèles URDF de bureau pour le catalogue de robots.
- **[HYDRA-UMC-DSI](https://github.com/JuanenRac/HYDRA-UMC-DSI)** — interface tactile native pour l'écran DSI embarqué.

**Plateforme URTC** — le contrôleur de tête d'outil que porte chaque bras HYDRA-UMC
- **[URTC](https://github.com/JuanenRac/URTC)** — contrôleur de tête d'outil sur bus CAN, 25 profils d'outil.
- **[URTC-FLASHER](https://github.com/JuanenRac/URTC-FLASHER)** — outil de bureau de flashage CAN-OTA + SWD/JTAG.
- **[URTC-TESTER](https://github.com/JuanenRac/URTC-TESTER)** — outil de bureau de diagnostic CAN en direct.
- **[URTC-WEB-STUDIO](https://github.com/JuanenRac/URTC-WEB-STUDIO)** — alternative basée navigateur via l'API Web Serial.

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


## 👤 AUTEUR
**JuanenRac** (Electro Hobby 3D)
📧 electrohobby3d@gmail.com

## 📜 LICENCE
GPL-3.0 - Voir LICENSE pour plus de détails.
