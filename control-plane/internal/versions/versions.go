// Package versions centralises all Docker image version strings used in generated bundles.
// Update this file when bumping image versions — both the CLI and the server will pick up the change.
package versions

const (
	// Shared infrastructure images
	Redis   = "redis:7-alpine"
	Foundry = "ghcr.io/foundry-rs/foundry:v1.5.1"

	// POPSigner / Localestia images (banhbao registry)
	PopSignerLite = "rg.nl-ams.scw.cloud/banhbao/popsigner-lite:v0.1.3"
	Localestia    = "rg.nl-ams.scw.cloud/banhbao/localestia:v0.1.5.1"

	// Nitro images (banhbao registry)
	NitroNode      = "rg.nl-ams.scw.cloud/banhbao/nitro-node-dev:v3.10.0"
	NitroDASServer = "rg.nl-ams.scw.cloud/banhbao/nitro-das-server:v0.8.2"

	// OP Stack images (banhbao registry)
	OpAltDA = "rg.nl-ams.scw.cloud/banhbao/op-alt-da:v0.10.1"

	// OP Stack images (oplabs public registry)
	OpGeth     = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101602.3"
	OpNode     = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.16.3"
	OpBatcher  = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:v1.16.3"
	OpProposer = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:v1.10.0"
)
