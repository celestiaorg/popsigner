/**
 * Tests for Nitro Orbit Chain Deployment
 */

import type { Address } from 'viem';
import {
  validateConfig,
  parseConfig,
} from './deploy';
import {
  type NitroDeploymentConfig,
  DeploymentConfigError,
} from './types';

// Test fixtures
const VALID_ADDRESS = '0x742d35Cc6634C0532925a3b844Bc454b332' as Address;
const ZERO_ADDRESS = '0x0000000000000000000000000000000000000000' as Address;

const TEST_CLIENT_CERT = `-----BEGIN CERTIFICATE-----
MIIBkTCB+wIJAKHBfpegZZMeMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RDQTAZMBUED0FJQUFBQUJBUk1BQUFBQUE9MAoGCCqGSM49BAMCMBExDzANBgNV
-----END CERTIFICATE-----`;

const TEST_CLIENT_KEY = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgevZzL1gdAFr88hb2
-----END PRIVATE KEY-----`;

const validConfig: NitroDeploymentConfig = {
  chainId: 42069,
  chainName: 'Test L3',
  parentChainId: 421614,
  parentChainRpc: 'https://sepolia-rollup.arbitrum.io/rpc',
  owner: VALID_ADDRESS,
  batchPosters: [VALID_ADDRESS],
  validators: [VALID_ADDRESS],
  stakeToken: ZERO_ADDRESS,
  baseStake: '100000000000000000',
  dataAvailability: 'celestia',
  popsignerEndpoint: 'https://rpc.popsigner.com:8546',
  clientCert: TEST_CLIENT_CERT,
  clientKey: TEST_CLIENT_KEY,
};

describe('validateConfig', () => {
  it('should accept valid configuration', () => {
    expect(() => validateConfig(validConfig)).not.toThrow();
  });

  describe('required fields', () => {
    const requiredFields: (keyof NitroDeploymentConfig)[] = [
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

    it.each(requiredFields)('should reject missing %s', (field) => {
      const config = { ...validConfig };
      delete (config as Record<string, unknown>)[field];

      expect(() => validateConfig(config as NitroDeploymentConfig)).toThrow(
        DeploymentConfigError,
      );
    });
  });

  describe('chainId validation', () => {
    it('should reject zero chainId', () => {
      expect(() =>
        validateConfig({ ...validConfig, chainId: 0 }),
      ).toThrow('chainId must be positive');
    });

    it('should reject negative chainId', () => {
      expect(() =>
        validateConfig({ ...validConfig, chainId: -1 }),
      ).toThrow('chainId must be positive');
    });
  });

  describe('batchPosters validation', () => {
    it('should reject empty batchPosters array', () => {
      expect(() =>
        validateConfig({ ...validConfig, batchPosters: [] }),
      ).toThrow('At least one batch poster required');
    });
  });

  describe('validators validation', () => {
    it('should reject empty validators array', () => {
      expect(() =>
        validateConfig({ ...validConfig, validators: [] }),
      ).toThrow('At least one validator required');
    });
  });

  describe('baseStake validation', () => {
    it('should reject invalid baseStake format', () => {
      expect(() =>
        validateConfig({ ...validConfig, baseStake: 'not-a-number' }),
      ).toThrow('baseStake must be a valid integer string');
    });

    it('should reject zero baseStake', () => {
      expect(() =>
        validateConfig({ ...validConfig, baseStake: '0' }),
      ).toThrow('baseStake must be positive');
    });

    it('should reject negative baseStake', () => {
      expect(() =>
        validateConfig({ ...validConfig, baseStake: '-100' }),
      ).toThrow('baseStake must be positive');
    });

    it('should accept valid baseStake', () => {
      expect(() =>
        validateConfig({ ...validConfig, baseStake: '1000000000000000000' }),
      ).not.toThrow();
    });
  });

  describe('dataAvailability validation', () => {
    it('should accept celestia (default)', () => {
      expect(() =>
        validateConfig({ ...validConfig, dataAvailability: 'celestia' }),
      ).not.toThrow();
    });

    it('should accept rollup (legacy)', () => {
      expect(() =>
        validateConfig({ ...validConfig, dataAvailability: 'rollup' }),
      ).not.toThrow();
    });

    it('should accept anytrust (legacy)', () => {
      expect(() =>
        validateConfig({ ...validConfig, dataAvailability: 'anytrust' }),
      ).not.toThrow();
    });

    it('should accept undefined dataAvailability (defaults to celestia)', () => {
      const config = { ...validConfig };
      delete (config as Record<string, unknown>).dataAvailability;
      expect(() => validateConfig(config as NitroDeploymentConfig)).not.toThrow();
    });

    it('should reject invalid dataAvailability', () => {
      expect(() =>
        validateConfig({
          ...validConfig,
          dataAvailability: 'invalid' as 'celestia',
        }),
      ).toThrow('dataAvailability must be');
    });
  });

  describe('popsignerEndpoint validation', () => {
    it('should reject non-HTTPS endpoint', () => {
      expect(() =>
        validateConfig({
          ...validConfig,
          popsignerEndpoint: 'http://rpc.popsigner.com:8546',
        }),
      ).toThrow('popsignerEndpoint must use HTTPS');
    });
  });
});

describe('parseConfig', () => {
  it('should parse valid JSON config', () => {
    const json = JSON.stringify(validConfig);
    const config = parseConfig(json);

    expect(config.chainId).toBe(42069);
    expect(config.chainName).toBe('Test L3');
    expect(config.dataAvailability).toBe('celestia');
  });

  it('should apply default values', () => {
    const minimalConfig = {
      chainId: 42069,
      chainName: 'Test L3',
      parentChainId: 421614,
      parentChainRpc: 'https://sepolia-rollup.arbitrum.io/rpc',
      owner: VALID_ADDRESS,
      batchPosters: [VALID_ADDRESS],
      validators: [VALID_ADDRESS],
      stakeToken: ZERO_ADDRESS,
      baseStake: '100000000000000000',
      dataAvailability: 'celestia',
      popsignerEndpoint: 'https://rpc.popsigner.com:8546',
      clientCert: TEST_CLIENT_CERT,
      clientKey: TEST_CLIENT_KEY,
    };

    const config = parseConfig(JSON.stringify(minimalConfig));

    expect(config.confirmPeriodBlocks).toBe(45818);
    expect(config.extraChallengeTimeBlocks).toBe(0);
    expect(config.maxDataSize).toBe(117964);
    expect(config.deployFactoriesToL2).toBe(true);
  });

  it('should allow overriding defaults', () => {
    const customConfig = {
      ...validConfig,
      confirmPeriodBlocks: 1000,
      extraChallengeTimeBlocks: 500,
      maxDataSize: 200000,
      deployFactoriesToL2: false,
    };

    const config = parseConfig(JSON.stringify(customConfig));

    expect(config.confirmPeriodBlocks).toBe(1000);
    expect(config.extraChallengeTimeBlocks).toBe(500);
    expect(config.maxDataSize).toBe(200000);
    expect(config.deployFactoriesToL2).toBe(false);
  });

  it('should throw on invalid JSON', () => {
    expect(() => parseConfig('not valid json')).toThrow(DeploymentConfigError);
  });

  it('should throw on empty JSON', () => {
    expect(() => parseConfig('')).toThrow(DeploymentConfigError);
  });

  it('should handle JSON with extra fields gracefully', () => {
    const configWithExtra = {
      ...validConfig,
      unknownField: 'some value',
      anotherExtra: 12345,
    };

    const config = parseConfig(JSON.stringify(configWithExtra));
    expect(config.chainId).toBe(42069);
  });
});

describe('NitroDeploymentConfig types', () => {
  it('should allow optional nativeToken', () => {
    const config: NitroDeploymentConfig = {
      ...validConfig,
      nativeToken: '0x1234567890123456789012345678901234567890' as Address,
    };
    expect(config.nativeToken).toBeDefined();
  });

  it('should allow optional caCert', () => {
    const config: NitroDeploymentConfig = {
      ...validConfig,
      caCert: '-----BEGIN CERTIFICATE-----...',
    };
    expect(config.caCert).toBeDefined();
  });
});

describe('DeploymentConfigError', () => {
  it('should store field name', () => {
    const error = new DeploymentConfigError('Test error', 'chainId');
    expect(error.name).toBe('DeploymentConfigError');
    expect(error.message).toBe('Test error');
    expect(error.field).toBe('chainId');
  });

  it('should work without field name', () => {
    const error = new DeploymentConfigError('Test error');
    expect(error.field).toBeUndefined();
  });
});

