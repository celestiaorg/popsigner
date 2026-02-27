package main

// Image versions for all Docker images used in generated bundles.
// Update these constants when publishing new image releases.
const (
	// Shared infrastructure
	imageRedis   = "redis:7-alpine"
	imageFoundry = "ghcr.io/foundry-rs/foundry:v1.5.1"

	// POPsigner / Celestia stack (shared between Nitro and OP Stack bundles)
	imagePopSignerLite = "rg.nl-ams.scw.cloud/banhbao/popsigner-lite:v0.1.3"
	imageLocalestia    = "rg.nl-ams.scw.cloud/banhbao/localestia:v0.1.5.1"

	// Nitro-specific
	imageNitroNode      = "rg.nl-ams.scw.cloud/banhbao/nitro-node-dev:v3.10.0"
	imageNitroDASServer = "rg.nl-ams.scw.cloud/banhbao/nitro-das-server:v0.8.2"

	// OP Stack-specific
	imageOpAltDA   = "rg.nl-ams.scw.cloud/banhbao/op-alt-da:v0.10.1"
	imageOpGeth    = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-geth:v1.101602.3"
	imageOpNode    = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-node:v1.16.3"
	imageOpBatcher = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-batcher:v1.16.3"
	imageOpProposer = "us-docker.pkg.dev/oplabs-tools-artifacts/images/op-proposer:v1.10.0"
)
