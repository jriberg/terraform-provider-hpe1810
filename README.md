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

Binaries are never committed. A release is built by goreleaser, GPG-signed and uploaded to the
registry:

```bash
git tag v0.1.0 && git push origin v0.1.0
GPG_FINGERPRINT=<fpr> GITHUB_TOKEN=<pat> goreleaser release --clean
# then: boring-registry upload provider --namespace phirna \
#   --filename-sha256sums $PWD/dist/terraform-provider-hpe1810_0.1.0_SHA256SUMS ...
```

The full flow (signing key, `signing-keys.json`, upload flags) is in ohana's
`nomad/boring-registry.md`.
`terraform-registry-manifest.json` declares plugin protocol 6.0 (terraform-plugin-framework).
