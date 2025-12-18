#!/usr/bin/env node
/**
 * Nitro Deployer CLI
 *
 * Command-line interface for deploying Arbitrum Nitro/Orbit chains.
 * Designed to be called as a subprocess by the Go orchestrator.
 *
 * Usage:
 *   node cli.js <config.json>     # Read config from file
 *   cat config.json | node cli.js # Read config from stdin
 *
 * Output:
 *   - Result JSON is written to stdout
 *   - Logs are written to stderr
 *   - Exit code 0 on success, 1 on failure
 *
 * @module cli
 */

import * as fs from 'fs';
import { deployOrbitChain, parseConfig } from './deploy';
import { DeploymentConfigError } from './types';

/**
 * Reads input from stdin.
 */
async function readStdin(): Promise<string> {
  const chunks: Buffer[] = [];
  
  return new Promise((resolve, reject) => {
    process.stdin.on('data', (chunk: Buffer) => {
      chunks.push(chunk);
    });
    
    process.stdin.on('end', () => {
      resolve(Buffer.concat(chunks).toString('utf-8'));
    });
    
    process.stdin.on('error', (err: Error) => {
      reject(err);
    });

    // Set a timeout for stdin read
    setTimeout(() => {
      if (chunks.length === 0) {
        reject(new Error('No input received from stdin within timeout'));
      }
    }, 5000);
  });
}

/**
 * Prints usage information.
 */
function printUsage(): void {
  console.error(`
Nitro Orbit Chain Deployer

Usage:
  node cli.js <config.json>       Deploy using config file
  cat config.json | node cli.js   Deploy using stdin input
  node cli.js --help              Show this help

Config JSON format:
{
  "chainId": 42069,
  "chainName": "My L3",
  "parentChainId": 421614,
  "parentChainRpc": "https://sepolia-rollup.arbitrum.io/rpc",
  "owner": "0x...",
  "batchPosters": ["0x..."],
  "validators": ["0x..."],
  "stakeToken": "0x0000000000000000000000000000000000000000",
  "baseStake": "100000000000000000",
  "dataAvailability": "celestia",  // optional, defaults to "celestia"
  "popsignerEndpoint": "https://rpc.popsigner.com:8546",
  "clientCert": "-----BEGIN CERTIFICATE-----...",
  "clientKey": "-----BEGIN PRIVATE KEY-----..."
}

Output:
  JSON result is written to stdout
  Logs are written to stderr
  Exit code 0 on success, 1 on failure
`);
}

/**
 * Main entry point.
 */
async function main(): Promise<void> {
  const args = process.argv.slice(2);

  // Handle help flag
  if (args.includes('--help') || args.includes('-h')) {
    printUsage();
    process.exit(0);
  }

  let configJson: string;

  try {
    if (args.length > 0 && args[0] !== '-') {
      // Read config from file argument
      const configPath = args[0];
      
      if (!fs.existsSync(configPath)) {
        throw new Error(`Config file not found: ${configPath}`);
      }
      
      configJson = fs.readFileSync(configPath, 'utf-8');
      console.error(`[nitro-deployer] Reading config from: ${configPath}`);
    } else if (!process.stdin.isTTY) {
      // Read config from stdin (piped input)
      console.error('[nitro-deployer] Reading config from stdin...');
      configJson = await readStdin();
    } else {
      // Interactive terminal with no file - show usage
      printUsage();
      process.exit(1);
    }

    // Parse and validate config
    const config = parseConfig(configJson);
    console.error(`[nitro-deployer] Config parsed successfully`);
    console.error(`[nitro-deployer] Chain: ${config.chainName} (ID: ${config.chainId})`);

    // Deploy the chain
    const result = await deployOrbitChain(config);

    // Output result as JSON to stdout
    console.log(JSON.stringify(result, null, 2));

    // Exit with appropriate code
    process.exit(result.success ? 0 : 1);
  } catch (error) {
    // Handle errors
    const errorMessage = error instanceof Error ? error.message : String(error);
    const isConfigError = error instanceof DeploymentConfigError;

    console.error(`[nitro-deployer] ${isConfigError ? 'Config error' : 'Fatal error'}: ${errorMessage}`);

    // Output error result as JSON
    console.log(
      JSON.stringify(
        {
          success: false,
          error: errorMessage,
        },
        null,
        2,
      ),
    );

    process.exit(1);
  }
}

// Run main
main().catch((error) => {
  console.error('[nitro-deployer] Unhandled error:', error);
  console.log(
    JSON.stringify(
      {
        success: false,
        error: error instanceof Error ? error.message : String(error),
      },
      null,
      2,
    ),
  );
  process.exit(1);
});

