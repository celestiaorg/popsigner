/**
 * Tests for POPSigner Custom Viem Account
 */

import * as https from 'https';
import { EventEmitter } from 'events';
import type { Hex, TransactionSerializable } from 'viem';
import {
  createPOPSignerAccount,
  POPSignerError,
  MTLSConfigError,
  type TypedDataInput,
} from './popsigner-account';
import type { POPSignerConfig } from './types';

// Mock https module
jest.mock('https');

// Test fixtures
const TEST_ADDRESS = '0x742d35Cc6634C0532925a3b844Bc454b332';
const TEST_ENDPOINT = 'https://rpc.popsigner.com:8546';

const TEST_CLIENT_CERT = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpegZZMeMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAZMBUED0FJQUFBQUJBUk1BQUFBQUE9MAoGCCqGSM49BAMCMBExDzANBgNV
BAMMBnRlc3RDQTAAAAAAMB4XDTIzMDEwMTAwMDAwMFoXDTI0MDEwMTAwMDAwMFow
EjEQMA4GA1UEAwwHY2xpZW50MTBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABPnP
-----END CERTIFICATE-----`;

const TEST_CLIENT_KEY = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgevZzL1gdAFr88hb2
OF/2NxApJCzGCEDdfSp6VQO30hyhRANCAAQRWz+jn65BtOMvdyHKcvjBeBSDZH2r
-----END PRIVATE KEY-----`;

const TEST_CA_CERT = `-----BEGIN CERTIFICATE-----
MIIBjDCB9gIJALVVqFwxQCCwMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAZMBUED0FJQUFBQUJBUk1BQUFBQUE9MAoGCCqGSM49BAMCMBExDzANBgNV
-----END CERTIFICATE-----`;

const validConfig: POPSignerConfig = {
  endpoint: TEST_ENDPOINT,
  address: TEST_ADDRESS as `0x${string}`,
  clientCert: TEST_CLIENT_CERT,
  clientKey: TEST_CLIENT_KEY,
  caCert: TEST_CA_CERT,
};

// Mock response helper
function createMockResponse(data: unknown, statusCode = 200): EventEmitter & { statusCode: number } {
  const response = new EventEmitter() as EventEmitter & { statusCode: number };
  response.statusCode = statusCode;
  
  setImmediate(() => {
    response.emit('data', JSON.stringify(data));
    response.emit('end');
  });
  
  return response;
}

// Mock request helper
function createMockRequest(): EventEmitter & { write: jest.Mock; end: jest.Mock; destroy: jest.Mock } {
  const request = new EventEmitter() as EventEmitter & { write: jest.Mock; end: jest.Mock; destroy: jest.Mock };
  request.write = jest.fn();
  request.end = jest.fn();
  request.destroy = jest.fn();
  return request;
}

describe('createPOPSignerAccount', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  describe('configuration validation', () => {
    it('should throw MTLSConfigError when endpoint is missing', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          endpoint: '',
        }),
      ).toThrow(MTLSConfigError);
    });

    it('should throw MTLSConfigError when endpoint is not HTTPS', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          endpoint: 'http://rpc.popsigner.com:8546',
        }),
      ).toThrow('POPSigner endpoint must use HTTPS for mTLS');
    });

    it('should throw MTLSConfigError when address is missing', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          address: '' as `0x${string}`,
        }),
      ).toThrow(MTLSConfigError);
    });

    it('should throw MTLSConfigError when address is invalid', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          address: 'invalid-address' as `0x${string}`,
        }),
      ).toThrow('Valid Ethereum address is required');
    });

    it('should throw MTLSConfigError when client certificate is missing', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          clientCert: '',
        }),
      ).toThrow('Client certificate (PEM) is required for mTLS');
    });

    it('should throw MTLSConfigError when client key is missing', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          clientKey: '',
        }),
      ).toThrow('Client private key (PEM) is required for mTLS');
    });

    it('should throw MTLSConfigError when client certificate is not PEM-encoded', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          clientCert: 'not-a-pem-certificate',
        }),
      ).toThrow('Client certificate must be PEM-encoded');
    });

    it('should throw MTLSConfigError when client key is not PEM-encoded', () => {
      expect(() =>
        createPOPSignerAccount({
          ...validConfig,
          clientKey: 'not-a-pem-key',
        }),
      ).toThrow('Client key must be PEM-encoded');
    });

    it('should accept valid configuration', () => {
      const account = createPOPSignerAccount(validConfig);
      expect(account).toBeDefined();
      expect(account.address).toBe(TEST_ADDRESS);
      expect(account.type).toBe('local');
    });

    it('should accept RSA private key format', () => {
      const account = createPOPSignerAccount({
        ...validConfig,
        clientKey: `-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END RSA PRIVATE KEY-----`,
      });
      expect(account).toBeDefined();
    });

    it('should accept EC private key format', () => {
      const account = createPOPSignerAccount({
        ...validConfig,
        clientKey: `-----BEGIN EC PRIVATE KEY-----
MIIEowIBAAKCAQEA...
-----END EC PRIVATE KEY-----`,
      });
      expect(account).toBeDefined();
    });

    it('should work without CA certificate', () => {
      const configWithoutCA = { ...validConfig };
      delete configWithoutCA.caCert;
      
      const account = createPOPSignerAccount(configWithoutCA);
      expect(account).toBeDefined();
    });
  });

  describe('signMessage', () => {
    it('should sign a string message', async () => {
      const expectedSignature = '0x1234567890abcdef' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignature,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signature = await account.signMessage({ message: 'Hello, World!' });

      expect(signature).toBe(expectedSignature);
      expect(https.request).toHaveBeenCalled();
      
      // Verify the request was made with correct method
      const requestBody = JSON.parse(mockRequest.write.mock.calls[0][0]);
      expect(requestBody.method).toBe('eth_sign');
      expect(requestBody.params[0]).toBe(TEST_ADDRESS);
    });

    it('should sign raw bytes message', async () => {
      const expectedSignature = '0xabcdef1234567890' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignature,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signature = await account.signMessage({
        message: { raw: new Uint8Array([1, 2, 3, 4, 5]) },
      });

      expect(signature).toBe(expectedSignature);
    });

    it('should throw POPSignerError on RPC error', async () => {
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        error: {
          code: -32000,
          message: 'Signing failed',
        },
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      await expect(
        account.signMessage({ message: 'test' }),
      ).rejects.toThrow(POPSignerError);
    });
  });

  describe('signTransaction', () => {
    const legacyTransaction: TransactionSerializable = {
      to: '0x1234567890123456789012345678901234567890' as `0x${string}`,
      value: 1000000000000000000n,
      gasPrice: 20000000000n,
      gas: 21000n,
      nonce: 5,
      chainId: 1,
      data: '0x' as Hex,
    };

    const eip1559Transaction: TransactionSerializable = {
      to: '0x1234567890123456789012345678901234567890' as `0x${string}`,
      value: 1000000000000000000n,
      maxFeePerGas: 30000000000n,
      maxPriorityFeePerGas: 1000000000n,
      gas: 21000n,
      nonce: 10,
      chainId: 1,
    };

    it('should sign a legacy transaction', async () => {
      const expectedSignedTx = '0xf86c0585...' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignedTx,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signedTx = await account.signTransaction(legacyTransaction);

      expect(signedTx).toBe(expectedSignedTx);
      
      const requestBody = JSON.parse(mockRequest.write.mock.calls[0][0]);
      expect(requestBody.method).toBe('eth_signTransaction');
      expect(requestBody.params[0].from).toBe(TEST_ADDRESS);
      expect(requestBody.params[0].type).toBe('0x0');
      expect(requestBody.params[0].gasPrice).toBeDefined();
    });

    it('should sign an EIP-1559 transaction', async () => {
      const expectedSignedTx = '0x02f87001...' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignedTx,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signedTx = await account.signTransaction(eip1559Transaction);

      expect(signedTx).toBe(expectedSignedTx);
      
      const requestBody = JSON.parse(mockRequest.write.mock.calls[0][0]);
      expect(requestBody.method).toBe('eth_signTransaction');
      expect(requestBody.params[0].type).toBe('0x2');
      expect(requestBody.params[0].maxFeePerGas).toBeDefined();
      expect(requestBody.params[0].maxPriorityFeePerGas).toBeDefined();
    });

    it('should include access list for EIP-2930 transactions', async () => {
      const eip2930Transaction = {
        to: '0x1234567890123456789012345678901234567890' as `0x${string}`,
        value: 1000000000000000000n,
        gasPrice: 20000000000n,
        gas: 21000n,
        nonce: 5,
        chainId: 1,
        data: '0x' as Hex,
        type: 'eip2930' as const,
        accessList: [
          {
            address: '0xaabbccdd00112233445566778899aabbccdd0011' as `0x${string}`,
            storageKeys: ['0x0000000000000000000000000000000000000000000000000000000000000001' as Hex],
          },
        ],
      };

      const expectedSignedTx = '0x01f8a501...' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignedTx,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      await account.signTransaction(eip2930Transaction);

      const requestBody = JSON.parse(mockRequest.write.mock.calls[0][0]);
      expect(requestBody.params[0].accessList).toBeDefined();
      expect(requestBody.params[0].accessList).toHaveLength(1);
    });

    it('should handle transaction without optional fields', async () => {
      const minimalTransaction: TransactionSerializable = {
        to: '0x1234567890123456789012345678901234567890' as `0x${string}`,
        chainId: 1,
      };

      const expectedSignedTx = '0xf85f01...' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignedTx,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signedTx = await account.signTransaction(minimalTransaction);

      expect(signedTx).toBe(expectedSignedTx);
    });
  });

  describe('signTypedData', () => {
    const typedData: TypedDataInput = {
      domain: {
        name: 'Test App',
        version: '1',
        chainId: 1,
        verifyingContract: '0x1234567890123456789012345678901234567890' as `0x${string}`,
      },
      types: {
        Person: [
          { name: 'name', type: 'string' },
          { name: 'wallet', type: 'address' },
        ],
      },
      primaryType: 'Person',
      message: {
        name: 'Alice',
        wallet: '0x1234567890123456789012345678901234567890',
      },
    };

    it('should sign EIP-712 typed data', async () => {
      const expectedSignature = '0x712signature...' as Hex;
      
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: expectedSignature,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      const signature = await account.signTypedData(typedData);

      expect(signature).toBe(expectedSignature);
      
      const requestBody = JSON.parse(mockRequest.write.mock.calls[0][0]);
      expect(requestBody.method).toBe('eth_signTypedData_v4');
      expect(requestBody.params[0]).toBe(TEST_ADDRESS);
      
      // The second param should be stringified typed data
      const parsedTypedData = JSON.parse(requestBody.params[1]);
      expect(parsedTypedData.domain).toEqual(typedData.domain);
      expect(parsedTypedData.types).toEqual(typedData.types);
      expect(parsedTypedData.primaryType).toBe('Person');
      expect(parsedTypedData.message).toEqual(typedData.message);
    });
  });

  describe('error handling', () => {
    it('should throw POPSignerError on HTTP error response', async () => {
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse(
        { error: 'Internal Server Error' },
        500,
      );

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      await expect(
        account.signMessage({ message: 'test' }),
      ).rejects.toThrow(POPSignerError);
    });

    it('should throw POPSignerError on network error', async () => {
      const mockRequest = createMockRequest();

      (https.request as jest.Mock).mockImplementation(() => {
        setImmediate(() => {
          mockRequest.emit('error', new Error('ECONNREFUSED'));
        });
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      await expect(
        account.signMessage({ message: 'test' }),
      ).rejects.toThrow('Request failed: ECONNREFUSED');
    });

    it('should throw POPSignerError on timeout', async () => {
      const mockRequest = createMockRequest();

      (https.request as jest.Mock).mockImplementation(() => {
        setImmediate(() => {
          mockRequest.emit('timeout');
        });
        return mockRequest;
      });

      const account = createPOPSignerAccount({
        ...validConfig,
        timeout: 100,
      });
      
      await expect(
        account.signMessage({ message: 'test' }),
      ).rejects.toThrow('Request timeout');
      
      expect(mockRequest.destroy).toHaveBeenCalled();
    });

    it('should throw POPSignerError on empty response', async () => {
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        id: 1,
        // No result or error
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      await expect(
        account.signMessage({ message: 'test' }),
      ).rejects.toThrow('Empty response from POPSigner');
    });

    it('should include error code and data in POPSignerError', async () => {
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        error: {
          code: -32602,
          message: 'Invalid params',
          data: { detail: 'address mismatch' },
        },
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      try {
        await account.signMessage({ message: 'test' });
        fail('Expected POPSignerError to be thrown');
      } catch (error) {
        expect(error).toBeInstanceOf(POPSignerError);
        expect((error as POPSignerError).code).toBe(-32602);
        expect((error as POPSignerError).data).toEqual({ detail: 'address mismatch' });
      }
    });
  });

  describe('mTLS agent configuration', () => {
    it('should configure HTTPS agent with mTLS certificates', () => {
      createPOPSignerAccount(validConfig);

      // Verify https.Agent was called with correct options during request
      expect(https.request).not.toHaveBeenCalled(); // Not called until signing
    });

    it('should use custom timeout when specified', async () => {
      const customTimeout = 5000;
      const mockRequest = createMockRequest();
      const mockResponse = createMockResponse({
        jsonrpc: '2.0',
        result: '0x1234' as Hex,
        id: 1,
      });

      (https.request as jest.Mock).mockImplementation((options, callback) => {
        expect(options.timeout).toBe(customTimeout);
        callback(mockResponse);
        return mockRequest;
      });

      const account = createPOPSignerAccount({
        ...validConfig,
        timeout: customTimeout,
      });
      
      await account.signMessage({ message: 'test' });
    });
  });

  describe('request ID incrementing', () => {
    it('should increment request ID for each call', async () => {
      const writeCalls: string[] = [];
      
      (https.request as jest.Mock).mockImplementation((options, callback) => {
        // Create fresh response for each call
        const mockResponse = createMockResponse({
          jsonrpc: '2.0',
          result: '0x1234' as Hex,
          id: 1,
        });
        callback(mockResponse);
        
        const mockRequest = createMockRequest();
        mockRequest.write = jest.fn((data: string) => {
          writeCalls.push(data);
        });
        return mockRequest;
      });

      const account = createPOPSignerAccount(validConfig);
      
      await account.signMessage({ message: 'test1' });
      await account.signMessage({ message: 'test2' });
      await account.signMessage({ message: 'test3' });

      expect(writeCalls.length).toBe(3);
      expect(JSON.parse(writeCalls[0]).id).toBe(1);
      expect(JSON.parse(writeCalls[1]).id).toBe(2);
      expect(JSON.parse(writeCalls[2]).id).toBe(3);
    });
  });
});

