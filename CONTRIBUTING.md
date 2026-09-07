# Contributing to HYDRA-UMC-TOOL-CLI 🦾

We welcome contributions to the powerful command-line interface of the HYDRA-UMC ecosystem.

## Technology Stack
- **Languages**: Go 1.22+ (Main CLI), Python 3.12 (Scripting logic).
- **Communication**: gRPC, WebSocket (via Cobra/Viper).
- **Architecture**: Modular command-based structure.
- **Environment**: Cross-platform (Windows, Linux, macOS).

## Guidelines
1. **Command Consistency**: All new commands must follow the project's naming convention (`hydra-cli <noun> <verb>`) and include high-quality `--help` documentation.
2. **Error Handling**: Implement robust error reporting for network timeouts and malformed protocol buffers.
3. **Security**: Ensure that sensitive credentials (like JWT tokens) are stored securely using system-specific keystores or environment variables.
4. **Testing**: Validate all CLI commands against the `HYDRA-UMC-TWIN` simulated swarm to ensure correct behavior without physical hardware.
