package secp256k1

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

// pathSign returns the path definitions for signing operations.
func pathSign(b *backend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "sign/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeString,
					Description: "Name of the key to use for signing",
					Required:    true,
				},
				"input": {
					Type:        framework.TypeString,
					Description: "Base64-encoded data to sign (will be hashed if not prehashed)",
					Required:    true,
				},
				"prehashed": {
					Type:        framework.TypeBool,
					Description: "If true, input is already a 32-byte hash. Otherwise, input will be hashed.",
					Default:     false,
				},
				"hash_algorithm": {
					Type:        framework.TypeString,
					Description: "Hash algorithm to use: sha256 (default) or keccak256",
					Default:     "sha256",
				},
				"output_format": {
					Type:        framework.TypeString,
					Description: "Signature output format: cosmos (default, R||S 64 bytes) or der",
					Default:     "cosmos",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.UpdateOperation: &framework.PathOperation{
					Callback:    b.pathSignWrite,
					Summary:     "Sign data with a secp256k1 key",
					Description: "Signs data using the specified key.",
				},
			},
			HelpSynopsis:    pathSignHelpSyn,
			HelpDescription: pathSignHelpDesc,
		},
	}
}

// pathSignWrite handles the sign operation.
func (b *backend) pathSignWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	if name == "" {
		return logical.ErrorResponse("missing key name"), nil
	}

	inputB64 := data.Get("input").(string)
	if inputB64 == "" {
		return logical.ErrorResponse("missing input"), nil
	}

	prehashed := data.Get("prehashed").(bool)
	hashAlgo := data.Get("hash_algorithm").(string)
	outputFormat := data.Get("output_format").(string)

	// Decode the input
	input, err := base64.StdEncoding.DecodeString(inputB64)
	if err != nil {
		return logical.ErrorResponse("invalid input: not valid base64"), nil
	}

	// Compute or validate hash
	var hash []byte
	if prehashed {
		if len(input) != 32 {
			return logical.ErrorResponse("prehashed input must be 32 bytes"), nil
		}
		hash = input
	} else {
		switch hashAlgo {
		case "sha256":
			hash = hashSHA256(input)
		case "keccak256":
			hash = hashKeccak256(input)
		default:
			return logical.ErrorResponse("unsupported hash algorithm: %s", hashAlgo), nil
		}
	}

	// Get the key
	entry, err := b.getKey(ctx, req.Storage, name)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return logical.ErrorResponse("key not found"), nil
	}

	// Parse the private key
	privKey, err := ParsePrivateKey(entry.PrivateKey)
	if err != nil {
		return nil, err
	}

	// Sign the hash
	sig, err := SignMessage(privKey, hash)
	if err != nil {
		return nil, err
	}

	// Format signature based on output format
	var sigOut []byte
	switch outputFormat {
	case "cosmos", "":
		sigOut = sig // Already in R||S format
	case "der":
		// Convert to DER format
		sigOut, err = cosmosSignatureToDER(sig)
		if err != nil {
			return nil, err
		}
	default:
		return logical.ErrorResponse("unsupported output format: %s", outputFormat), nil
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"signature":   base64.StdEncoding.EncodeToString(sigOut),
			"public_key":  hex.EncodeToString(entry.PublicKey),
			"key_version": 1,
		},
	}, nil
}

// cosmosSignatureToDER converts R||S format to DER format.
func cosmosSignatureToDER(cosmosSignature []byte) ([]byte, error) {
	if len(cosmosSignature) != 64 {
		return nil, fmt.Errorf("invalid cosmos signature length: %d", len(cosmosSignature))
	}

	sig, err := parseCosmosSignature(cosmosSignature)
	if err != nil {
		return nil, err
	}

	return sig.Serialize(), nil
}

// parseDERSignature parses a DER-encoded signature.
func parseDERSignature(der []byte) (*ecdsa.Signature, error) {
	return ecdsa.ParseDERSignature(der)
}

const pathSignHelpSyn = `Sign data with a secp256k1 key`

const pathSignHelpDesc = `
This endpoint signs data using the specified secp256k1 key.

The input should be provided as a base64-encoded string. By default, the input
will be hashed with SHA-256 before signing. You can specify 'prehashed=true'
if the input is already a 32-byte hash.

Parameters:
  input          - Base64-encoded data to sign
  prehashed      - If true, input is already a 32-byte hash (default: false)
  hash_algorithm - Hash algorithm: sha256 (default) or keccak256
  output_format  - Signature format: cosmos (default) or der

Examples:
  # Sign raw data with SHA-256 hash:
  $ bao write secp256k1/sign/mykey input="<base64-encoded-data>"

  # Sign with Keccak-256 (Ethereum):
  $ bao write secp256k1/sign/mykey input="<base64-data>" hash_algorithm=keccak256

  # Sign prehashed data:
  $ bao write secp256k1/sign/mykey input="<base64-32-byte-hash>" prehashed=true

  # Get DER-encoded signature:
  $ bao write secp256k1/sign/mykey input="<base64-data>" output_format=der

Response:
  signature   - Base64-encoded signature (64 bytes R||S for cosmos, DER for der)
  public_key  - Hex-encoded compressed public key (33 bytes)
  key_version - Key version (always 1)
`
