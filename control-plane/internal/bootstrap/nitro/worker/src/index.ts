/**
 * Nitro Deployer Worker
 *
 * Node.js package for deploying Arbitrum Nitro chains using POPSigner
 * for secure remote signing via mTLS.
 *
 * @packageDocumentation
 */

export {
  createPOPSignerAccount,
  createPOPSignerAccountFromFiles,
} from './popsigner-account';

export type {
  POPSignerAccount,
  TypedDataInput,
} from './popsigner-account';

export type {
  POPSignerConfig,
  JSONRPCRequest,
  JSONRPCResponse,
  JSONRPCError,
  TransactionParams,
  AccessListItem,
  TypedDataDomain,
} from './types';

export { POPSignerError, MTLSConfigError } from './types';
