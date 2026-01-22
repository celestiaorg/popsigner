// Package repository provides data access layer for bootstrap deployment persistence.
package repository

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Stack represents the chain stack type (OP Stack or Nitro).
type Stack string

const (
	// StackOPStack represents the Optimism Stack deployment type.
	StackOPStack Stack = "opstack"
	// StackNitro represents the Arbitrum Nitro deployment type.
	StackNitro Stack = "nitro"
	// StackPopBundle represents the POPKins Devnet Bundle type (OP Stack + Celestia DA).
	StackPopBundle Stack = "pop-bundle"
)

// Status represents the deployment status.
type Status string

const (
	// StatusPending indicates the deployment has not started.
	StatusPending Status = "pending"
	// StatusRunning indicates the deployment is in progress.
	StatusRunning Status = "running"
	// StatusPaused indicates the deployment is paused (can be resumed).
	StatusPaused Status = "paused"
	// StatusCompleted indicates the deployment finished successfully with real on-chain transactions.
	StatusCompleted Status = "completed"
	// StatusSimulated indicates the deployment ran in simulation mode (no real contracts deployed).
	StatusSimulated Status = "simulated"
	// StatusFailed indicates the deployment failed.
	StatusFailed Status = "failed"
)

// Deployment represents a chain deployment record.
type Deployment struct {
	ID           uuid.UUID
	OrgID        uuid.UUID // Organization that owns this deployment
	ChainID      int64
	Stack        Stack
	Status       Status
	CurrentStage *string
	Config       json.RawMessage
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Transaction represents a blockchain transaction recorded during deployment.
type Transaction struct {
	ID           uuid.UUID
	DeploymentID uuid.UUID
	Stage        string
	TxHash       string
	Description  *string
	CreatedAt    time.Time
}

// Artifact represents a deployment artifact (genesis, configs, etc.).
type Artifact struct {
	ID           uuid.UUID
	DeploymentID uuid.UUID
	ArtifactType string
	Content      json.RawMessage
	CreatedAt    time.Time
}

