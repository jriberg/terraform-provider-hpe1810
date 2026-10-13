package main

import (
	"context"
	"log"

	tfprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"terraform-provider-hpe1810/internal/provider"
)

// version is overridden at build time via -ldflags, e.g.:
//
//	go build -ldflags "-X main.version=0.1.0"
var version = "dev"

func main() {
	p := provider.New(version)
	err := providerserver.Serve(context.Background(), func() tfprovider.Provider { return p }, providerserver.ServeOpts{
		// Must match the source address consumers use in required_providers
		// -- served by the self-hosted boring-registry (ohana gitea #74).
		// For local dev with dev_overrides, use this same address as the
		// key in ~/.tofurc's dev_overrides block.
		Address: "registry.phirna.uk/phirna/hpe1810",
	})
	// Best-effort: optionally save the running config (if save_running_config=true
	// and something actually changed this run), then always free up the switch's
	// one session slot so the next `tofu` invocation doesn't have to wait out the
	// switch's own session timeout.
	p.Shutdown()
	if err != nil {
		log.Fatal(err.Error())
	}
}
