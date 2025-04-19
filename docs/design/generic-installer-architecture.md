---
title: "InstallSpec‑Driven Installer Architecture"
date: "2025-04-19"
author: "goinstaller Team"
version: "0.2.0"
status: "draft"
---

# Generic Config‑Driven Installer Architecture

> 📄 **Document series overview**  
> This file lays out the *architecture* of a generic, config‑driven installer
> pipeline.  The concrete specification of the on‑disk schema referred to in
> this document lives in **[InstallSpec v1](install-spec-v1.md)** which should be
> read together with this file.

## 1. Background & Motivation
Today, `goinstaller` only supports reading a GoReleaser YAML (`.goreleaser.yml`) to generate a shell installer script.
Many projects either do not use GoReleaser or have custom asset naming conventions and release workflows.
We need a pluggable, data‑source‑agnostic architecture that—given minimal inputs via CLI flags, manual config, GitHub API, etc.—can generate the same installer logic without being tightly coupled to GoReleaser.

## 2. Goals
- Extract a **pure InstallSpec** that fully describes name, version placeholder, supported platforms, asset templates, checksum/verification settings, etc.
- Define a **SourceAdapter** interface to populate that InstallSpec from any origin.
- Maintain a single **ScriptGenerator** component that transforms InstallSpec → installer code (shell, PowerShell, …).
- Preserve existing GoReleaser flow as one SourceAdapter implementation.

## 3. High‑Level Architecture

```text
┌──────────────┐    ┌───────────────┐    ┌───────────────────┐
│ SourceAdapter│ ─> │  InstallSpec │ ─> │  ScriptBuilder   │ ─> install.sh
└──────────────┘    └───────────────┘    └───────────────────┘
```

- **SourceAdapter**: interface `Detect(ctx, in DetectInput) (InstallSpec, error)`
  - goreleaserAdapter (existing `.goreleaser.yml`)
  - githubProbeAdapter (GitHub Releases API, asset inspection)
  - flagsAdapter (CLI flags for name, patterns, etc.)
  - fileAdapter (user‑supplied `install-spec.yaml`)
- **InstallSpec**: Go struct/YAML schema that holds:
  - `name`, `repo`, `version` placeholder
  - per‑platform archives (`os`, `arch`, `asset` template, `bin`)
  - checksum definition (file, algorithm, embedded checksums)
  - attestation verification settings
  - unpack options (strip components)
  - For detailed schema definition, see [InstallSpec v1](install-spec-v1.md)
- **ScriptBuilder**: generates installer scripts
  - powered by Go `text/template` (+ sprig)
  - supports POSIX shell & PowerShell; template sets are pluggable
  - injects download, checksum verify, attestation, retry, flags

## 4. Two-Step Workflow

The new architecture introduces a two-step workflow to simplify the process:

1. **Config Generation**: First generate the InstallSpec config
   ```bash
   goinstaller init-config --source [goreleaser|github|cli] [options]
   ```

2. **Script Generation**: Then generate the installer script from the config
   ```bash
   goinstaller generate-script --config install-spec.yaml [options]
   ```

Additionally, a utility command is provided to embed checksums into an existing config:

```bash
goinstaller embed-checksums --config install-spec.yaml --checksum-file SHA256SUMS
```

This separation allows for:
- Better validation and inspection of the config before script generation
- Reuse of configs across multiple script generations
- Simplified script generation logic
- Easier testing and debugging
- Ability to add checksums to existing configs from external checksum files

## 5. CLI UX Examples

> **Note**: These examples represent the current design thinking and may evolve during implementation. The exact command names, flags, and syntax are subject to change.

```bash
# Step 1: Generate config from GoReleaser
goinstaller init-config \
  --source goreleaser \
  --file .goreleaser.yml \
  --output install-spec.yaml

# Step 1: Generate config from GitHub Releases
goinstaller init-config \
  --source github \
  --repo owner/repo \
  --tag v1.2.3 \
  --asset-pattern "{{name}}_{{version}}_{{os}}_{{arch}}.tar.gz" \
  --output install-spec.yaml

# Step 1: Generate config from CLI flags
goinstaller init-config \
  --source cli \
  --name mytool \
  --version 0.9.0 \
  --base-url https://example.com/downloads \
  --asset linux/amd64=mytool_{{version}}_linux_amd64.tgz \
  --output install-spec.yaml

# Optional: Embed checksums into an existing config
goinstaller embed-checksums \
  --config install-spec.yaml \
  --checksum-file SHA256SUMS \
  --version v1.2.3

# Step 2: Generate script from config
goinstaller generate-script \
  --config install-spec.yaml \
  --output install.sh
```

## 6. Integration with Existing Code
```
goinstaller/
├── cmd/goinstaller/main.go       # add subcommands for init-config and generate-script
├── internal/
│   ├── datasource/               # new package
│   │   ├── interface.go          # SourceAdapter interface & options
│   │   ├── goreleaser.go         # existing logic moved here
│   │   ├── github.go             # GitHub probe implementation
│   │   ├── flags.go              # flags → InstallSpec
│   │   └── file.go               # install-spec.yaml parser
│   ├── config/                   # InstallSpec struct + YAML schema
│   └── shell/                    # existing generator refactored as ScriptBuilder
│       ├── generator.go
│       └── templates/
└── pkg/
    └── verify/                   # checksum & attestation helpers
```

## 7. Embedded Checksums Benefits

The embedded checksums feature provides several significant advantages:

### 7.1 Performance Improvements
- **Reduced HTTP Requests**: Eliminates the need to download separate checksum files during installation, reducing the number of HTTP requests by at least one per installation.
- **Faster Installations**: Installation completes more quickly, especially on slower networks, as there's no need to wait for additional checksum file downloads.
- **Optimized Verification Flow**: When the installer script itself is verified with attestation, the embedded checksums can be trusted implicitly, allowing the binary verification process to be streamlined or even skipped in certain scenarios, further accelerating the installation process.

### 7.2 Reliability Enhancements
- **Offline Installation Support**: Enables completely offline installations once the installer script and binary are downloaded, as no additional network requests for checksum files are needed.
- **Reduced Network Dependency**: Less susceptible to temporary network issues or checksum file server unavailability.
- **Consistent Verification**: Ensures the same checksums are used for verification regardless of network conditions or changes to remote checksum files.

### 7.3 Security Considerations
- **Pre-verified Integrity**: Checksums can be pre-verified by the script generator using `gh attestation verify` or other secure methods before embedding.
- **Tamper Resistance**: Makes it harder for attackers to substitute malicious checksums, as they would need to modify the installer script itself (which could also be signed or verified).
- **Audit Trail**: Provides a clear record of which checksums were used for verification at the time the installer was generated.
- **Trust Chain**: When the installer script is verified with attestation, the embedded checksums inherit this trust, creating a stronger end-to-end security model.

### 7.4 User Experience
- **Simplified Installation**: Users don't need to worry about checksum file availability or format.
- **Consistent Behavior**: Installation process behaves the same way across different environments and network conditions.
- **Transparent Verification**: Users can inspect the embedded checksums in the installer script before running it.

This feature is particularly valuable for enterprise environments with strict security policies, air-gapped systems, or deployments in regions with unreliable internet connectivity.

## 8. Implementation Roadmap
1. Phase 1: Extract InstallSpec & SourceAdapter interface; migrate current GoReleaser code
2. Phase 2: Implement two-step workflow with init-config and generate-script commands
3. Phase 3: Implement GitHub Probe adapter (API calls, naming heuristics)
4. Phase 4: Add File & Flags adapters
5. Phase 5: Implement embed-checksums command for adding checksums to existing configs
6. Phase 6: Update templates, tests, examples, docs

## 9. Risks & Mitigations
- Naming inference may require pattern flags for edge cases
- GitHub rate limits: support unauthenticated vs token flows
- Template versioning: allow per‑project overrides and locking
- Embedded checksums: ensure proper validation of pre-verified checksums
