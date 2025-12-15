package popsigner

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// AuditService handles audit log operations.
type AuditService struct {
	client *Client
}

// AuditFilter specifies filters for querying audit logs.
type AuditFilter struct {
	// Event filters by event type.
	Event *AuditEvent
	// ResourceType filters by resource type.
	ResourceType *ResourceType
	// ResourceID filters by resource ID.
	ResourceID *uuid.UUID
	// ActorID filters by actor ID.
	ActorID *uuid.UUID
	// StartTime filters logs after this time.
	StartTime *time.Time
	// EndTime filters logs before this time.
	EndTime *time.Time
	// Limit is the maximum number of logs to return (max 100).
	Limit int
	// Cursor is the pagination cursor from a previous response.
	Cursor string
}

// AuditListResponse is the response from listing audit logs.
type AuditListResponse struct {
	// Logs is the list of audit logs.
	Logs []*AuditLog
	// NextCursor is the cursor for the next page, empty if no more pages.
	NextCursor string
}

// List retrieves audit logs with optional filters.
//
// Example:
//
//	// Get all audit logs
//	resp, err := client.Audit.List(ctx, nil)
//
//	// Get logs for a specific key
//	keyID := uuid.MustParse("...")
//	resp, err := client.Audit.List(ctx, &popsigner.AuditFilter{
//	    ResourceType: popsigner.Ptr(popsigner.ResourceTypeKey),
//	    ResourceID:   &keyID,
//	})
//
//	// Paginate through results
//	resp, _ := client.Audit.List(ctx, nil)
//	for resp.NextCursor != "" {
//	    resp, _ = client.Audit.List(ctx, &popsigner.AuditFilter{Cursor: resp.NextCursor})
//	}
func (s *AuditService) List(ctx context.Context, filter *AuditFilter) (*AuditListResponse, error) {
	// Build query parameters
	params := url.Values{}
	if filter != nil {
		if filter.Event != nil {
			params.Set("event", string(*filter.Event))
		}
		if filter.ResourceType != nil {
			params.Set("resource_type", string(*filter.ResourceType))
		}
		if filter.ResourceID != nil {
			params.Set("resource_id", filter.ResourceID.String())
		}
		if filter.ActorID != nil {
			params.Set("actor_id", filter.ActorID.String())
		}
		if filter.StartTime != nil {
			params.Set("start_time", filter.StartTime.Format(time.RFC3339))
		}
		if filter.EndTime != nil {
			params.Set("end_time", filter.EndTime.Format(time.RFC3339))
		}
		if filter.Limit > 0 {
			params.Set("limit", fmt.Sprintf("%d", filter.Limit))
		}
		if filter.Cursor != "" {
			params.Set("cursor", filter.Cursor)
		}
	}

	path := "/v1/audit/logs"
	if len(params) > 0 {
		path = fmt.Sprintf("%s?%s", path, params.Encode())
	}

	var resp struct {
		Data []*auditLogResponse `json:"data"`
		Meta *struct {
			NextCursor string `json:"next_cursor,omitempty"`
		} `json:"meta,omitempty"`
	}

	if err := s.client.get(ctx, path, &resp); err != nil {
		return nil, err
	}

	logs := make([]*AuditLog, len(resp.Data))
	for i, log := range resp.Data {
		logs[i] = log.toAuditLog()
	}

	result := &AuditListResponse{
		Logs: logs,
	}
	if resp.Meta != nil {
		result.NextCursor = resp.Meta.NextCursor
	}

	return result, nil
}

// Get retrieves a specific audit log by ID.
//
// Example:
//
//	log, err := client.Audit.Get(ctx, logID)
func (s *AuditService) Get(ctx context.Context, logID uuid.UUID) (*AuditLog, error) {
	var resp struct {
		Data auditLogResponse `json:"data"`
	}
	if err := s.client.get(ctx, fmt.Sprintf("/v1/audit/logs/%s", logID), &resp); err != nil {
		return nil, err
	}
	return resp.Data.toAuditLog(), nil
}

// auditLogResponse is the internal API response format for audit logs.
type auditLogResponse struct {
	ID           uuid.UUID              `json:"id"`
	OrgID        uuid.UUID              `json:"org_id"`
	Event        AuditEvent             `json:"event"`
	ActorID      *uuid.UUID             `json:"actor_id,omitempty"`
	ActorType    ActorType              `json:"actor_type"`
	ResourceType *ResourceType          `json:"resource_type,omitempty"`
	ResourceID   *uuid.UUID             `json:"resource_id,omitempty"`
	IPAddress    *string                `json:"ip_address,omitempty"`
	UserAgent    *string                `json:"user_agent,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    string                 `json:"created_at"`
}

// toAuditLog converts an auditLogResponse to an AuditLog.
func (r *auditLogResponse) toAuditLog() *AuditLog {
	createdAt, _ := time.Parse(time.RFC3339, r.CreatedAt)

	return &AuditLog{
		ID:           r.ID,
		OrgID:        r.OrgID,
		Event:        r.Event,
		ActorID:      r.ActorID,
		ActorType:    r.ActorType,
		ResourceType: r.ResourceType,
		ResourceID:   r.ResourceID,
		IPAddress:    r.IPAddress,
		UserAgent:    r.UserAgent,
		Metadata:     r.Metadata,
		CreatedAt:    createdAt,
	}
}

// Ptr is a helper function to create a pointer to a value.
// Useful for setting optional filter parameters.
//
// Example:
//
//	filter := &popsigner.AuditFilter{
//	    Event: popsigner.Ptr(popsigner.AuditEventKeySigned),
//	}
func Ptr[T any](v T) *T {
	return &v
}
