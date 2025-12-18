/**
 * Nitro Orbit Chain Deployment
 *
 * Deploys a new Arbitrum Nitro/Orbit chain using the orbit-sdk.
 * Uses POPSigner for mTLS-authenticated transaction signing.
 *
 * @module deploy
 */

import {
  createPublicClient,
  createWalletClient,
  http,
  defineChain,
  type Address,
  type Hex,
  type PublicClient,
  type WalletClient,
  type Transport,
  type Chain,
} from 'viem';
import { arbitrum, arbitrumSepolia } from 'viem/chains';
import {
  createRollupPrepareDeploymentParamsConfig,
  createRollupPrepareTransactionRequest,
  prepareChainConfig,
  createRollupPrepareTransactionReceipt,
} from '@arbitrum/orbit-sdk';

import { createPOPSignerAccount, type POPSignerAccount } from './popsigner-account';
import {
  type NitroDeploymentConfig,
  type DeploymentResult,
  type CoreContracts,
  DeploymentConfigError,
  DeploymentError,
} from './types';

/**
 * Zero address constant.
 */
const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000' as Address;

/**
 * Default deployment parameters.
 */
const DEFAULTS = {
  confirmPeriodBlocks: 45818, // ~1 week on Ethereum
  extraChallengeTimeBlocks: 0,
  maxDataSize: 117964,
  deployFactoriesToL2: true,
};

/**
 * Logs a message to stderr (for debugging, not captured by Go wrapper).
 */
function log(message: string): void {
  console.error(`[nitro-deployer] ${message}`);
}

/**
 * Validates the deployment configuration.
 * @throws {DeploymentConfigError} If configuration is invalid.
 */
export function validateConfig(config: NitroDeploymentConfig): void {
  const required: (keyof NitroDeploymentConfig)[] = [
    'chainId',
    'chainName',
    'parentChainId',
    'parentChainRpc',
    'owner',
    'batchPosters',
    'validators',
    'stakeToken',
    'baseStake',
    // dataAvailability is optional - defaults to 'celestia'
    'popsignerEndpoint',
    'clientCert',
    'clientKey',
  ];

  for (const field of required) {
    if (config[field] === undefined || config[field] === null) {
      throw new DeploymentConfigError(`Missing required field: ${field}`, field);
    }
  }

  // Validate chainId
  if (config.chainId <= 0) {
    throw new DeploymentConfigError('chainId must be positive', 'chainId');
  }

  // Validate arrays have at least one element
  if (config.batchPosters.length === 0) {
    throw new DeploymentConfigError('At least one batch poster required', 'batchPosters');
  }

  if (config.validators.length === 0) {
    throw new DeploymentConfigError('At least one validator required', 'validators');
  }

  // Validate baseStake is a valid bigint string
  try {
    const stake = BigInt(config.baseStake);
    if (stake <= 0n) {
      throw new DeploymentConfigError('baseStake must be positive', 'baseStake');
    }
  } catch (error) {
    // Re-throw our own errors
    if (error instanceof DeploymentConfigError) {
      throw error;
    }
    throw new DeploymentConfigError('baseStake must be a valid integer string', 'baseStake');
  }

  // Validate dataAvailability - default to celestia if not provided or invalid
  // POPSigner deployments use Celestia DA by default
  if (config.dataAvailability && !['rollup', 'anytrust', 'celestia'].includes(config.dataAvailability)) {
    throw new DeploymentConfigError(
      'dataAvailability must be "celestia", "rollup", or "anytrust"',
      'dataAvailability',
    );
  }

  // Validate POPSigner endpoint
  if (!config.popsignerEndpoint.startsWith('https://')) {
    throw new DeploymentConfigError(
      'popsignerEndpoint must use HTTPS',
      'popsignerEndpoint',
    );
  }
}

/**
 * Gets the Viem chain definition for a given chain ID.
 */
function getParentChain(chainId: number, rpcUrl: string): Chain {
  switch (chainId) {
    case 42161:
      return arbitrum;
    case 421614:
      return arbitrumSepolia;
    default:
      // For other chains, create a custom chain config
      return defineChain({
        id: chainId,
        name: `Chain ${chainId}`,
        network: `chain-${chainId}`,
        nativeCurrency: { name: 'Ether', symbol: 'ETH', decimals: 18 },
        rpcUrls: {
          default: { http: [rpcUrl] },
          public: { http: [rpcUrl] },
        },
      });
  }
}

/**
 * Deploys a new Nitro/Orbit chain.
 *
 * This is an atomic operation - the entire chain is deployed in a single transaction.
 * Uses the POPSigner Viem account for mTLS-authenticated signing.
 *
 * @param config - Deployment configuration
 * @returns Deployment result with contract addresses or error
 */
export async function deployOrbitChain(
  config: NitroDeploymentConfig,
): Promise<DeploymentResult> {
  try {
    // Validate configuration
    validateConfig(config);
    
    log(`Deploying Orbit chain ${config.chainName} (ID: ${config.chainId})`);
    log(`Parent chain: ${config.parentChainId}`);
    log(`Data availability: ${config.dataAvailability}`);

    // Get parent chain definition
    const parentChain = getParentChain(config.parentChainId, config.parentChainRpc);

    // Create POPSigner account for signing
    const account = createPOPSignerAccount({
      endpoint: config.popsignerEndpoint,
      address: config.owner,
      clientCert: config.clientCert,
      clientKey: config.clientKey,
      caCert: config.caCert,
      timeout: 60000, // 60 second timeout for deployment
    });

    log(`Using deployer address: ${account.address}`);

    // Create public client for reading chain state
    const publicClient = createPublicClient({
      chain: parentChain,
      transport: http(config.parentChainRpc),
    });

    // Prepare chain configuration
    // DataAvailabilityCommittee is true for external DA (Celestia/AnyTrust)
    // POPSigner deployments always use Celestia DA
    const dataAvailability = config.dataAvailability || 'celestia';
    const chainConfig = prepareChainConfig({
      chainId: config.chainId,
      arbitrum: {
        InitialChainOwner: config.owner,
        DataAvailabilityCommittee: dataAvailability !== 'rollup',
      },
    });

    log('Chain config prepared');

    // Prepare deployment parameters using orbit-sdk
    const deploymentConfig = await createRollupPrepareDeploymentParamsConfig(publicClient, {
      chainId: BigInt(config.chainId),
      owner: config.owner,
      chainConfig,
    });

    log('Deployment config prepared');

    // Prepare the deployment transaction request
    const txRequest = await createRollupPrepareTransactionRequest({
      params: {
        config: deploymentConfig,
        batchPosters: config.batchPosters,
        validators: config.validators,
        nativeToken: config.nativeToken ?? ZERO_ADDRESS,
        deployFactoriesToL2: config.deployFactoriesToL2 ?? DEFAULTS.deployFactoriesToL2,
        maxDataSize: BigInt(config.maxDataSize ?? DEFAULTS.maxDataSize),
      },
      account: account.address,
      publicClient,
    });

    log('Transaction request prepared');
    log('Sending deployment transaction...');

    // Create wallet client for sending transactions
    // We need to cast the account to satisfy viem's types
    const walletClient = createWalletClient({
      account: account as unknown as `0x${string}`,
      chain: parentChain,
      transport: http(config.parentChainRpc),
    });

    // Send the deployment transaction using the POPSigner account
    // We manually sign and send since our account type doesn't match viem's exactly
    const signedTx = await account.signTransaction({
      ...txRequest,
      chainId: parentChain.id,
    });

    // Broadcast the signed transaction
    const txHash = await publicClient.sendRawTransaction({
      serializedTransaction: signedTx,
    });

    log(`Transaction submitted: ${txHash}`);
    log('Waiting for confirmation...');

    // Wait for transaction receipt
    const receipt = await publicClient.waitForTransactionReceipt({
      hash: txHash,
      confirmations: 2,
      timeout: 300_000, // 5 minute timeout
    });

    log(`Transaction confirmed in block ${receipt.blockNumber}`);

    // Check transaction status
    if (receipt.status !== 'success') {
      throw new DeploymentError('Transaction reverted', txHash);
    }

    // Parse contract addresses from receipt using orbit-sdk helper
    const txReceipt = createRollupPrepareTransactionReceipt(receipt);
    const coreContracts = txReceipt.getCoreContracts();

    log('Deployment successful!');
    log(`Rollup address: ${coreContracts.rollup}`);
    log(`Inbox address: ${coreContracts.inbox}`);

    return {
      success: true,
      coreContracts: {
        rollup: coreContracts.rollup,
        inbox: coreContracts.inbox,
        outbox: coreContracts.outbox,
        bridge: coreContracts.bridge,
        sequencerInbox: coreContracts.sequencerInbox,
        rollupEventInbox: coreContracts.rollupEventInbox,
        challengeManager: coreContracts.challengeManager,
        adminProxy: coreContracts.adminProxy,
        upgradeExecutor: coreContracts.upgradeExecutor,
        validatorWalletCreator: coreContracts.validatorWalletCreator,
        nativeToken: coreContracts.nativeToken ?? ZERO_ADDRESS,
        deployedAtBlockNumber: Number(receipt.blockNumber),
      },
      transactionHash: txHash,
      blockNumber: Number(receipt.blockNumber),
      chainConfig: chainConfig as Record<string, unknown>,
    };
  } catch (error) {
    const errorMessage = error instanceof Error ? error.message : String(error);
    const txHash = error instanceof DeploymentError ? error.transactionHash : undefined;

    log(`Deployment failed: ${errorMessage}`);

    return {
      success: false,
      error: errorMessage,
      transactionHash: txHash,
    };
  }
}

/**
 * Parses deployment config from a JSON string.
 * @throws {DeploymentConfigError} If JSON is invalid.
 */
export function parseConfig(json: string): NitroDeploymentConfig {
  try {
    const config = JSON.parse(json) as Partial<NitroDeploymentConfig>;
    
    // Apply defaults - POPSigner uses Celestia DA by default
    return {
      confirmPeriodBlocks: DEFAULTS.confirmPeriodBlocks,
      extraChallengeTimeBlocks: DEFAULTS.extraChallengeTimeBlocks,
      maxDataSize: DEFAULTS.maxDataSize,
      deployFactoriesToL2: DEFAULTS.deployFactoriesToL2,
      // Override with provided values
      ...config,
      // Ensure dataAvailability defaults to celestia if not provided
      dataAvailability: config.dataAvailability || 'celestia',
    } as NitroDeploymentConfig;
  } catch (error) {
    throw new DeploymentConfigError(
      `Invalid JSON: ${error instanceof Error ? error.message : String(error)}`,
    );
  }
}
