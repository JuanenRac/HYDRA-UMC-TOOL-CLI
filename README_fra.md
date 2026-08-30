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
* ✅ **`hydra-cli doctor [--server URL]`** — diagnostic de contrat du serveur en lecture seule : valide `/api/hydra-info` et `/api/settings`, puis compare les totaux publiés de contrôleurs/robots avec la liste. N'envoie aucune commande et ne sonde aucun matériel. *(implémenté)*
* ✅ **Un contrat de codes de sortie réel et stable** — `0` ok, `1` erreur générale, `2` erreur d'usage, `3` erreur de configuration, `4` erreur réseau, `5` erreur serveur, `6` non implémenté. Chaque commande classe ses propres échecs à travers ce contrat plutôt qu'un simple `exit 1`, afin que les scripts encapsulant cette CLI puissent se brancher selon *pourquoi* elle a échoué. *(implémenté)*
* ✅ **`hydra-cli config validate --config PATH`** — charge et valide par schéma un fichier de configuration local (URL du serveur, délai des requêtes). *(implémenté)*
* ✅ **`hydra-cli config apply --config PATH [--dry-run]`** — `--dry-run` prouve le vrai chemin de validation de bout en bout et affiche exactement ce qu'elle enverrait ; sans lui, elle retourne honnêtement "non implémenté" car aucun endpoint d'écriture de flotte en direct n'existe encore. *(implémenté, dry-run uniquement)*
* ✅ **`hydra-cli shell [--server URL]`** — un REPL interactif : exécute n'importe laquelle des commandes ci-dessus de façon répétée contre le même serveur sans redémarrer le processus. Passe par exactement la même table de commandes que les invocations ponctuelles, si bien que le comportement du shell et celui en une seule fois ne peuvent jamais diverger. `exit`/`quit`/Ctrl-D pour quitter. *(implémenté)*
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
* **Pourquoi `robots` lit `GET /api/settings` plutôt qu'un nouvel endpoint.** Cet endpoint porte déjà la liste complète des contrôleurs/robots et est déjà une vraie lecture réelle, non authentifiée (voir le propre `src/server.ts` de HYDRA-UMC-SERVER) - `robots` est un vrai client d'un contrat déjà livré, pas un nouveau travail côté serveur. `doctor` utilise cette même lecture avec `/api/hydra-info` pour détecter des totaux publics incompatibles sans ajouter d'endpoint. Les commandes plus importantes, encore prévues (`deploy`/`flash-all`/`audit` orienté matériel), ont réellement besoin de nouveaux endpoints d'écriture qui n'existent pas encore.
* **Pourquoi `doctor` est explicitement en lecture seule.** Un contrôle de contrat est utile avant de disposer du matériel et sûr en CI. Il ne rapporte que la cohérence HTTP/JSON/compteurs ; il ne commande aucun équipement et ne prétend pas vérifier la santé CAN, actionneurs, capteurs, caméras, Hailo, CM5 ou sécurité.
* **Pourquoi `config apply` sans `--dry-run` retourne "non implémenté" plutôt que de ne rien faire silencieusement.** L'endpoint d'écriture en direct de HYDRA-UMC-SERVER que cela appellerait n'existe vraiment pas encore (le même manque qui bloque `deploy`/`flash-all`) - un code de sortie distinct, `ExitNotImplemented`, indique à l'appelant "c'est un vrai manque, pas un bug", plutôt qu'un no-op faussement réussi.
* **Pourquoi les erreurs de chaque commande passent désormais par un seul type `CliError`/`ExitCode` plutôt que par des appels ad hoc à `os.Exit`.** Un contrat de codes de sortie stable et documenté ne le reste que s'il existe un seul endroit qui attribue les codes - `exitCodeFor()` (`exitcode.go`) est cet endroit, et les fonctions de commande continuent de retourner des valeurs `error` idiomatiques et enchaînables plutôt que d'appeler elles-mêmes `os.Exit`.

---

## 📂 STRUCTURE DES DOSSIERS

Service purement logiciel (CLI) — sans matériel, micrologiciel ou système d'exploitation propres ; ces dossiers sont omis conformément à la politique de structure du dépôt.

```text
HYDRA-UMC-TOOL-CLI/
├── src/                       # Module Go
│   ├── go.mod                 # Définition du module (github.com/JuanenRac/HYDRA-UMC-TOOL-CLI)
│   └── cmd/hydra-cli/         # Point d'entrée du binaire
│       ├── main.go            # Répartition des commandes (version/help/status/robots/doctor/config)
│       ├── server.go          # Résolution partagée de --server/HYDRA_CLI_SERVER
│       ├── robots.go          # Vrai client GET /api/settings + affichage de la liste
│       ├── doctor.go          # Diagnostic de contrat à deux endpoints, en lecture seule
│       ├── config.go          # Chargement réel du fichier de configuration, validation, apply --dry-run
│       ├── exitcode.go        # Contrat réel et stable ExitCode/CliError
│       ├── *_test.go          # Vrais tests (allers-retours net/http/httptest, fixtures de fichiers temporaires)
│       └── version.go         # const Version = "0.0.0"
├── docs/                      # Documentation : CLI_REFERENCE.md et DOCTOR.md
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
./run.sh doctor --server http://localhost:3000
./run.sh config validate --config ./hydra-cli.json
./run.sh config apply --config ./hydra-cli.json --dry-run
echo $?   # 0=ok 2=usage 3=config 4=réseau 5=serveur 6=non-implémenté

# Windows
build.bat
run.bat version
run.bat status --server http://localhost:3000
run.bat robots --server http://localhost:3000
run.bat doctor --server http://localhost:3000
run.bat config validate --config .\hydra-cli.json
run.bat config apply --config .\hydra-cli.json --dry-run
```

`build` incrémente la version (`src/cmd/hydra-cli/version.go`), exécute la vraie suite de tests (`go vet` + `go test`), compile le module Go dans `src/` vers `build/hydra-cli(.exe)`, puis exécute `version` une fois pour vérifier. `run` relance le binaire compilé en transmettant tous les arguments — essayez `run doctor` face à une instance `HYDRA-UMC-SERVER` en cours d'exécution. Doctor est une vérification sûre et en lecture seule des contrats d'endpoints ; voir [docs/DOCTOR.md](docs/DOCTOR.md).

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

## 🛠️ BUILD & RUN

Utilisez la vérification de compilation sans versionnement avant une compilation de publication :

| Action | Windows | Linux / macOS |
|---|---|---|
| Vérification de compilation (sans modifier la version ni le CHANGELOG) | `build-test.bat` | `./build-test.sh` |
| Exécution / développement (si disponible) | `run*.bat` ou `dev*.bat` | `./run*.sh` ou `./dev*.sh` |

`build-test.bat` et `build-test.sh` compilent ou valident la pile du projet sans incrémenter `hydra-umc.project.json` ni modifier `CHANGELOG.md`. Ils peuvent uniquement créer les sorties normales du compilateur. Les scripts existants `build*.bat`, `build*.sh`, `run*` et `dev*` conservent leur comportement spécifique de versionnement ou d'exécution ; utilisez-les lorsque ce comportement est requis.
