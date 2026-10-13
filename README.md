# terraform-provider-hpe1810

OpenTofu/Terraform provider for the **HP 1810G (J9450A)** web-managed switch. It drives the
switch's own web UI over HTTP(S), since the switch has no API.

- **Resources:** `hpe1810_port`, `hpe1810_locator`, `hpe1810_green_features`,
  `hpe1810_system_description`, `hpe1810_time_zone`, `hpe1810_daylight_saving_time`,
  `hpe1810_lldp_configuration`, `hpe1810_flow_control`, `hpe1810_port_mirroring`, `hpe1810_trunk`
- **Data sources:** `system_description`, `network_setup`, `port_summary`, `lldp_statistics`,
  `buffered_log`, `mac_table`, `trunk_status`, `loop_protection_status`, `dual_image_status`,
  `clock`, `sntp`, `log_configuration`, `backup_manager`

Python prototype and page snapshots: <https://git.phirna.uk/public/hp-iac>.

## Usage

Published to a self-hosted [boring-registry](https://github.com/boring-registry/boring-registry)
(LAN-only, token auth):

```hcl
terraform {
  required_providers {
    hpe1810 = {
      source  = "registry.phirna.uk/phirna/hpe1810"
      version = "~> 0.1"
    }
  }
}

provider "hpe1810" {
  host                = "192.168.2.10" # or HPE1810_HOST
  https               = false          # or HPE1810_HTTPS
  password            = var.password   # or HPE1810_PASSWORD
  save_running_config = false          # or HPE1810_SAVE_RUNNING_CONFIG
  admin_port          = 24             # never written to (uplink/management guard)
}
```

The token goes in `~/.tofurc` as `credentials "registry.phirna.uk" { token = "..." }`, or in
`TF_TOKEN_registry_phirna_uk`.

### Gotchas

- The switch allows **one web session at a time**. Close the switch UI in your browser before
  running `tofu plan`. The provider logs out on exit (`p.Shutdown()` in `main.go`).
- Go's `net/http` honours `HTTP_PROXY`. If login returns `502`, set `NO_PROXY=<switch-ip>`.

## Development

```bash
go build -o terraform-provider-hpe1810 .
```

`~/.tofurc`:

```hcl
provider_installation {
  dev_overrides {
    "registry.phirna.uk/phirna/hpe1810" = "/path/to/this/repo"
  }
  direct {}
}
```

Skip `tofu init` with dev_overrides and go straight to `tofu plan`.

## Releasing

Binaries are never committed. `scripts/release.sh` does the whole flow. It tags and pushes, unlocks the
GPG key, builds and signs with goreleaser, uploads `signing-keys.json`, uploads to boring-registry and
checks that the version is listed:

```bash
scripts/release.sh 1.0.1                       # new release from a clean, pushed main
UPLOAD_ONLY=true scripts/release.sh 1.0.0      # tag + dist/ already exist; just upload
PUBLISH_GITHUB=true GITHUB_TOKEN=<pat> scripts/release.sh 1.0.1   # also create a GitHub release
```

Requirements: `git gpg goreleaser vault aws docker dig curl`, and a logged-in Vault (S3 key and API
token come from `secret/nomad/boring-registry`). S3 credentials are passed per command and never
exported, because tofu's S3 backend reads the same `AWS_*` variables.

Background and setup are in ohana's `nomad/boring-registry.md`.
`terraform-registry-manifest.json` declares plugin protocol 6.0 (terraform-plugin-framework).
