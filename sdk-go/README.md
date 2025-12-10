# BanhBaoRing Go SDK

Official Go SDK for the [BanhBaoRing](https://banhbaoring.io) Control Plane API.

BanhBaoRing is a secure key management and signing service backed by OpenBao, designed for blockchain applications like Celestia sequencers.

## Installation

```bash
go get github.com/banhbaoring/sdk-go
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/banhbaoring/sdk-go"
    "github.com/google/uuid"
)

func main() {
    // Create a client with your API key
    client := banhbaoring.NewClient("bbr_live_xxxxx")
    ctx := context.Background()

    // Create a key
    key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
        Name:        "my-sequencer-key",
        NamespaceID: uuid.MustParse("your-namespace-id"),
        Algorithm:   "secp256k1",
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Created key: %s (address: %s)\n", key.Name, key.Address)

    // Sign a message
    result, err := client.Sign.Sign(ctx, key.ID, []byte("Hello, World!"), false)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Signature: %x\n", result.Signature)
}
```

## Features

- **Key Management**: Create, list, get, delete, import, and export keys
- **Signing**: Sign messages and batch sign for parallel operations
- **Organizations**: Manage organizations, members, and namespaces
- **Audit Logs**: Query audit logs with filtering and pagination
- **Error Handling**: Typed errors with helper methods

## Client Options

```go
// Custom base URL
client := banhbaoring.NewClient(apiKey, banhbaoring.WithBaseURL("https://custom.api.io"))

// Custom timeout
client := banhbaoring.NewClient(apiKey, banhbaoring.WithTimeout(60*time.Second))

// Custom HTTP client
httpClient := &http.Client{
    Transport: &http.Transport{MaxIdleConns: 100},
}
client := banhbaoring.NewClient(apiKey, banhbaoring.WithHTTPClient(httpClient))
```

## Key Management

### Create a Key

```go
key, err := client.Keys.Create(ctx, banhbaoring.CreateKeyRequest{
    Name:        "sequencer-main",
    NamespaceID: namespaceID,
    Algorithm:   "secp256k1",  // or "ed25519"
    Exportable:  true,
    Metadata: map[string]string{
        "environment": "production",
    },
})
```

### Batch Create Keys (Parallel Workers Pattern)

```go
// Create 4 worker keys in parallel - perfect for Celestia sequencers
keys, err := client.Keys.CreateBatch(ctx, banhbaoring.CreateBatchRequest{
    Prefix:      "blob-worker",
    Count:       4,
    NamespaceID: namespaceID,
})
// Creates: blob-worker-1, blob-worker-2, blob-worker-3, blob-worker-4
```

### List Keys

```go
// List all keys
keys, err := client.Keys.List(ctx, nil)

// List keys in a specific namespace
keys, err := client.Keys.List(ctx, &banhbaoring.ListOptions{
    NamespaceID: &namespaceID,
})
```

### Import/Export Keys

```go
// Import a private key
key, err := client.Keys.Import(ctx, banhbaoring.ImportKeyRequest{
    Name:        "imported-key",
    NamespaceID: namespaceID,
    PrivateKey:  base64PrivateKey,  // base64-encoded
    Exportable:  true,
})

// Export a key (must be created with Exportable: true)
result, err := client.Keys.Export(ctx, keyID)
privateKey := result.PrivateKey  // base64-encoded
```

## Signing

### Sign a Message

```go
result, err := client.Sign.Sign(ctx, keyID, []byte("message"), false)
fmt.Printf("Signature: %x\n", result.Signature)
```

### Sign Pre-hashed Data

```go
// For blockchain transactions that require signing a hash
result, err := client.Sign.Sign(ctx, keyID, txHash, true)
```

### Batch Sign (Parallel Workers)

The batch sign API is critical for Celestia's parallel blob submission pattern:

```go
results, err := client.Sign.SignBatch(ctx, banhbaoring.BatchSignRequest{
    Requests: []banhbaoring.SignRequest{
        {KeyID: worker1, Data: tx1},
        {KeyID: worker2, Data: tx2},
        {KeyID: worker3, Data: tx3},
        {KeyID: worker4, Data: tx4},
    },
})
// All 4 signatures complete in ~200ms, not 800ms (4x speedup)!

for i, r := range results {
    if r.Error != "" {
        log.Printf("Worker %d failed: %s", i, r.Error)
    } else {
        fmt.Printf("Worker %d signature: %x\n", i, r.Signature)
    }
}
```

## Organizations

```go
// Create an organization
org, err := client.Orgs.Create(ctx, banhbaoring.CreateOrgRequest{
    Name: "My Organization",
})

// Create a namespace
ns, err := client.Orgs.CreateNamespace(ctx, orgID, banhbaoring.CreateNamespaceRequest{
    Name:        "production",
    Description: "Production keys",
})

// Invite a member
invitation, err := client.Orgs.InviteMember(ctx, orgID, banhbaoring.InviteMemberRequest{
    Email: "user@example.com",
    Role:  banhbaoring.RoleOperator,
})

// Get plan limits
limits, err := client.Orgs.GetLimits(ctx, orgID)
fmt.Printf("Keys: %d/%d\n", limits.CurrentKeys, limits.MaxKeys)
```

## Audit Logs

```go
// List all audit logs
resp, err := client.Audit.List(ctx, nil)

// Filter by event type
resp, err := client.Audit.List(ctx, &banhbaoring.AuditFilter{
    Event: banhbaoring.Ptr(banhbaoring.AuditEventKeySigned),
    Limit: 50,
})

// Filter by resource
keyID := uuid.MustParse("...")
resp, err := client.Audit.List(ctx, &banhbaoring.AuditFilter{
    ResourceType: banhbaoring.Ptr(banhbaoring.ResourceTypeKey),
    ResourceID:   &keyID,
})

// Paginate through results
filter := &banhbaoring.AuditFilter{Limit: 100}
for {
    resp, err := client.Audit.List(ctx, filter)
    if err != nil {
        log.Fatal(err)
    }
    
    for _, log := range resp.Logs {
        fmt.Printf("%s: %s\n", log.CreatedAt, log.Event)
    }
    
    if resp.NextCursor == "" {
        break
    }
    filter.Cursor = resp.NextCursor
}
```

## Error Handling

The SDK provides typed errors with helper methods:

```go
key, err := client.Keys.Get(ctx, keyID)
if err != nil {
    if apiErr, ok := banhbaoring.IsAPIError(err); ok {
        switch {
        case apiErr.IsNotFound():
            fmt.Println("Key not found")
        case apiErr.IsUnauthorized():
            fmt.Println("Invalid API key")
        case apiErr.IsForbidden():
            fmt.Println("Insufficient permissions")
        case apiErr.IsRateLimited():
            fmt.Println("Rate limit exceeded, retry later")
        case apiErr.IsValidationError():
            fmt.Printf("Validation error: %s\n", apiErr.Message)
        default:
            fmt.Printf("API error: %s (%s)\n", apiErr.Message, apiErr.Code)
        }
    } else {
        fmt.Printf("Network error: %v\n", err)
    }
}
```

## Examples

See the [examples](./examples) directory:

- [Basic Usage](./examples/basic/main.go) - Key management and signing
- [Parallel Workers](./examples/parallel-workers/main.go) - Batch operations for Celestia

## API Reference

### Client

| Method | Description |
|--------|-------------|
| `NewClient(apiKey, ...opts)` | Create a new client |
| `WithBaseURL(url)` | Set custom API URL |
| `WithTimeout(duration)` | Set HTTP timeout |
| `WithHTTPClient(client)` | Set custom HTTP client |

### KeysService

| Method | Description |
|--------|-------------|
| `Create(ctx, req)` | Create a new key |
| `CreateBatch(ctx, req)` | Create multiple keys |
| `Get(ctx, keyID)` | Get a key by ID |
| `List(ctx, opts)` | List all keys |
| `Delete(ctx, keyID)` | Delete a key |
| `Import(ctx, req)` | Import a private key |
| `Export(ctx, keyID)` | Export a key's private material |

### SignService

| Method | Description |
|--------|-------------|
| `Sign(ctx, keyID, data, prehashed)` | Sign data |
| `SignBatch(ctx, req)` | Sign multiple messages in parallel |

### OrgsService

| Method | Description |
|--------|-------------|
| `Create(ctx, req)` | Create an organization |
| `Get(ctx, orgID)` | Get an organization |
| `List(ctx)` | List organizations |
| `Update(ctx, orgID, req)` | Update an organization |
| `Delete(ctx, orgID)` | Delete an organization |
| `GetLimits(ctx, orgID)` | Get plan limits |
| `ListMembers(ctx, orgID)` | List members |
| `InviteMember(ctx, orgID, req)` | Invite a member |
| `RemoveMember(ctx, orgID, userID)` | Remove a member |
| `ListNamespaces(ctx, orgID)` | List namespaces |
| `CreateNamespace(ctx, orgID, req)` | Create a namespace |
| `GetNamespace(ctx, orgID, nsID)` | Get a namespace |
| `DeleteNamespace(ctx, orgID, nsID)` | Delete a namespace |

### AuditService

| Method | Description |
|--------|-------------|
| `List(ctx, filter)` | List audit logs with optional filters |
| `Get(ctx, logID)` | Get a specific audit log |

## License

MIT License - see [LICENSE](./LICENSE) for details.

