package secp256k1

import (
	"context"
	"encoding/base64"
	"encoding/hex"

	"github.com/openbao/openbao/sdk/v2/framework"
	"github.com/openbao/openbao/sdk/v2/logical"
)

// pathVerify returns the path definitions for signature verification.
func pathVerify(b *backend) []*framework.Path {
	return []*framework.Path{
		{
			Pattern: "verify/" + framework.GenericNameRegex("name"),
			Fields: map[string]*framework.FieldSchema{
				"name": {
					Type:        framework.TypeString,
					Description: "Name of the key to use for verification",
					Required:    true,
				},
				"input": {
					Type:        framework.TypeString,
					Description: "Base64-encoded data that was signed",
					Required:    true,
				},
				"signature": {
					Type:        framework.TypeString,
					Description: "Base64-encoded signature to verify",
					Required:    true,
				},
				"prehashed": {
					Type:        framework.TypeBool,
					Description: "If true, input is already a 32-byte hash",
					Default:     false,
				},
				"hash_algorithm": {
					Type:        framework.TypeString,
					Description: "Hash algorithm used: sha256 (default) or keccak256",
					Default:     "sha256",
				},
			},
			Operations: map[logical.Operation]framework.OperationHandler{
				logical.UpdateOperation: &framework.PathOperation{
					Callback:    b.pathVerifyWrite,
					Summary:     "Verify a signature with a secp256k1 key",
					Description: "Verifies a signature against data using the specified key.",
				},
			},
			HelpSynopsis:    pathVerifyHelpSyn,
			HelpDescription: pathVerifyHelpDesc,
		},
	}
}

// pathVerifyWrite handles the verify operation.
func (b *backend) pathVerifyWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {
	name := data.Get("name").(string)
	if name == "" {
		return logical.ErrorResponse("missing key name"), nil
	}

	inputB64 := data.Get("input").(string)
	if inputB64 == "" {
		return logical.ErrorResponse("missing input"), nil
	}

	sigB64 := data.Get("signature").(string)
	if sigB64 == "" {
		return logical.ErrorResponse("missing signature"), nil
	}

	prehashed := data.Get("prehashed").(bool)
	hashAlgo := data.Get("hash_algorithm").(string)

	// Decode the input
	input, err := base64.StdEncoding.DecodeString(inputB64)
	if err != nil {
		return logical.ErrorResponse("invalid input: not valid base64"), nil
	}

	// Decode the signature
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		return logical.ErrorResponse("invalid signature: not valid base64"), nil
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

	// Parse the public key
	pubKey, err := ParsePublicKey(entry.PublicKey)
	if err != nil {
		return nil, err
	}

	// Try to parse as cosmos signature (64 bytes) first
	valid := false
	if len(sig) == 64 {
		var verifyErr error
		valid, verifyErr = VerifySignature(pubKey, hash, sig)
		if verifyErr != nil {
			valid = false
		}
	} else {
		// Try DER format
		derSig, err := parseDERSignature(sig)
		if err != nil {
			return &logical.Response{
				Data: map[string]interface{}{
					"valid":      false,
					"public_key": hex.EncodeToString(entry.PublicKey),
				},
			}, nil
		}
		valid = derSig.Verify(hash, pubKey)
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"valid":      valid,
			"public_key": hex.EncodeToString(entry.PublicKey),
		},
	}, nil
}

const pathVerifyHelpSyn = `Verify a signature with a secp256k1 key`

const pathVerifyHelpDesc = `
This endpoint verifies a signature against data using the specified secp256k1 key.

Both the input and signature should be provided as base64-encoded strings.
The signature can be in either Cosmos format (64 bytes R||S) or DER format.

Parameters:
  input          - Base64-encoded data that was signed
  signature      - Base64-encoded signature to verify
  prehashed      - If true, input is already a 32-byte hash (default: false)
  hash_algorithm - Hash algorithm used: sha256 (default) or keccak256

Example:
  $ bao write secp256k1/verify/mykey \
      input="<base64-data>" \
      signature="<base64-signature>"

Response:
  valid      - true if signature is valid, false otherwise
  public_key - Hex-encoded compressed public key (33 bytes)
`
