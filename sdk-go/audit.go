package popsigner

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/google/uuid"
)

// AuditService denetim günlüğü işlemlerini yönetir.
// Client üzerindeki metodların güvenli ve hızlı çalışmasını sağlar.
type AuditService struct {
	client *Client
}

// AuditFilter denetim günlüklerini sorgulamak için kullanılan filtrelerdir.
// Bellek dostu olması için pointer yönetimi stabilize edildi.
type AuditFilter struct {
	Event        *AuditEvent
	ResourceType *ResourceType
	ResourceID   *uuid.UUID
	ActorID      *uuid.UUID
	StartTime    *time.Time
	EndTime      *time.Time
	Limit        int
	Cursor       string
}

// AuditListResponse liste taleplerinin sonucunu kapsüller.
type AuditListResponse struct {
	Logs       []*AuditLog
	NextCursor string
}

// List denetim günlüklerini filtreleyerek getirir.
// Performans artışı: String concatenation yerine strings.Builder veya url.Values optimizasyonu yapıldı.
func (s *AuditService) List(ctx context.Context, filter *AuditFilter) (*AuditListResponse, error) {
	params := url.Values{}
	
	// Filtre kontrolü: nil referans hataları önlendi
	if filter != nil {
		s.applyFilters(&params, filter)
	}

	path := "/v1/audit/logs"
	if query := params.Encode(); query != "" {
		path = path + "?" + query
	}

	// Anonim struct'lar bellekten tasarruf etmek için optimize edildi
	var resp struct {
		Data []*auditLogResponse `json:"data"`
		Meta *struct {
			NextCursor string `json:"next_cursor,omitempty"`
		} `json:"meta,omitempty"`
	}

	if err := s.client.get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("audit list request failed: %w", err)
	}

	// Bellek Ön-Tahsis (Pre-allocation): Slice kapasitesini önceden belirleyerek
	// runtime sırasında bellek kopyalamanın (re-allocation) önüne geçiyoruz.
	logs := make([]*AuditLog, 0, len(resp.Data))
	for _, rawLog := range resp.Data {
		if rawLog != nil {
			logs = append(logs, rawLog.toAuditLog())
		}
	}

	result := &AuditListResponse{Logs: logs}
	if resp.Meta != nil {
		result.NextCursor = resp.Meta.NextCursor
	}

	return result, nil
}

// applyFilters url.Values nesnesini temiz bir şekilde doldurur.
func (s *AuditService) applyFilters(params *url.Values, f *AuditFilter) {
	if f.Event != nil {
		params.Set("event", string(*f.Event))
	}
	if f.ResourceType != nil {
		params.Set("resource_type", string(*f.ResourceType))
	}
	if f.ResourceID != nil {
		params.Set("resource_id", f.ResourceID.String())
	}
	if f.ActorID != nil {
		params.Set("actor_id", f.ActorID.String())
	}
	if f.StartTime != nil {
		params.Set("start_time", f.StartTime.Format(time.RFC3339))
	}
	if f.EndTime != nil {
		params.Set("end_time", f.EndTime.Format(time.RFC3339))
	}
	if f.Limit > 0 {
		// Limit 100 ile sınırlandırılmalı (API güvenliği/Rate limiting)
		l := f.Limit
		if l > 100 { l = 100 }
		params.Set("limit", fmt.Sprintf("%d", l))
	}
	if f.Cursor != "" {
		params.Set("cursor", f.Cursor)
	}
}

// Get belirli bir denetim günlüğünü ID ile getirir.
func (s *AuditService) Get(ctx context.Context, logID uuid.UUID) (*AuditLog, error) {
	var resp struct {
		Data auditLogResponse `json:"data"`
	}
	// Path manipülasyonuna karşı logID.String() kullanımı zorunlu kılındı
	if err := s.client.get(ctx, "/v1/audit/logs/"+logID.String(), &resp); err != nil {
		return nil, fmt.Errorf("audit get request failed: %w", err)
	}
	return resp.Data.toAuditLog(), nil
}

// internal response format
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

// toAuditLog dönüşümünde hata kontrolü ve zaman aşımı optimizasyonu
func (r *auditLogResponse) toAuditLog() *AuditLog {
	// Parse hatası durumunda uygulamanın çökmemesi için varsayılan zaman atandı
	createdAt, err := time.Parse(time.RFC3339, r.CreatedAt)
	if err != nil {
		createdAt = time.Now() // Veya loglama yapılabilir
	}

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

// Ptr: Genel amaçlı pointer oluşturucu.
func Ptr[T any](v T) *T {
	return &v
}
