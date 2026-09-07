# Security Policy 🔒 (HYDRA-UMC-TOOL-CLI)

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.x.x  | ✅ Yes             |

## Reporting a Vulnerability

**CRITICAL: Do not report safety-critical vulnerabilities through public GitHub issues.**

In a management CLI, a security flaw can allow unauthorized factory-wide firmware updates or mission hijacking. If you discover a vulnerability affecting the **local token storage**, **CAN-OTA hijacking**, or **command spoofing**:

1. **Email**: Send a detailed report to `electrohobby3d@gmail.com`.
2. **Impact**: Describe if the bug allows unauthorized flashing of controllers, leaking swarm telemetry to the terminal, or bypassing CI/CD deployment safeguards.
3. **Response**: Initial acknowledgment within 48 hours.

We follow a coordinated disclosure policy to ensure hardware safety before public release.
