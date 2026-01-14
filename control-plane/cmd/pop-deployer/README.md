# pop-deployer

CLI tool that creates a pre-deployed OP Stack + Celestia DA devnet bundle.

## Usage

```bash
# Build the tool
go build -o pop-deployer .

# Run with default settings (uses /tmp/pop-deployer-bundle)
./pop-deployer

# Run with custom bundle directory (useful for debugging)
./pop-deployer -bundle-dir ./bundle

# View all options
./pop-deployer -h
```

## What It Does

1. Starts ephemeral Anvil (L1)
2. Starts popsigner-lite (signing service)
3. Deploys OP Stack contracts to Anvil
4. Captures Anvil state with pre-deployed contracts
5. Generates all configuration files (genesis.json, rollup.json, etc.)
6. Creates a `tar.gz` bundle ready for deployment

## Output

The tool creates:
- **Temporary build directory** (default: `/tmp/pop-deployer-bundle`)
  - Auto-cleaned on each run
  - Contains intermediate files
  - Auto-removed by OS tmpwatch/reboot

- **opstack-local-devnet-bundle.tar.gz** (in current directory)
  - The final deliverable
  - Extract and run with `docker compose up`
  - Contains everything needed for a working devnet

## Configuration Flags

- `-bundle-dir` - Where to write temporary build files
  - Default: `/tmp/pop-deployer-bundle`
  - Override for debugging: `-bundle-dir ./bundle`
  - Directory is removed and recreated on each run

## Why /tmp by Default?

The build directory is temporary and gets packaged into a tar.gz. Using `/tmp`:
- Follows Unix conventions for ephemeral build artifacts
- Auto-cleaned by system tmpwatch/tmpfiles
- No .gitignore maintenance needed
- Consistent with cache directory pattern

Override with `-bundle-dir ./bundle` if you need to inspect build artifacts for debugging.

## Integration

This tool is used by [scripts_popsignerlight/2-test-pop-deployer.sh](../../../../scripts_popsignerlight/2-test-pop-deployer.sh) for automated testing.

## Development Notes

- Build artifacts are intentionally temporary
- The tar.gz bundle is the only persistent output
- For production/website integration, consider extracting this into a service
