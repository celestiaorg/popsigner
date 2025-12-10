package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Bidon15/banhbaoring/control-plane/internal/models"
	apierrors "github.com/Bidon15/banhbaoring/control-plane/internal/pkg/errors"
)

// --- Mock Repositories ---

type mockKeyRepo struct {
	keys     map[uuid.UUID]*models.Key
	byOrgKey map[string]*models.Key // orgID_namespaceID_name -> key
}

func newMockKeyRepo() *mockKeyRepo {
	return &mockKeyRepo{
		keys:     make(map[uuid.UUID]*models.Key),
		byOrgKey: make(map[string]*models.Key),
	}
}

func (m *mockKeyRepo) Create(ctx context.Context, key *models.Key) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	key.CreatedAt = time.Now()
	key.UpdatedAt = key.CreatedAt
	m.keys[key.ID] = key
	keyStr := key.OrgID.String() + "_" + key.NamespaceID.String() + "_" + key.Name
	m.byOrgKey[keyStr] = key
	return nil
}

func (m *mockKeyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Key, error) {
	return m.keys[id], nil
}

func (m *mockKeyRepo) GetByName(ctx context.Context, orgID, namespaceID uuid.UUID, name string) (*models.Key, error) {
	keyStr := orgID.String() + "_" + namespaceID.String() + "_" + name
	return m.byOrgKey[keyStr], nil
}

func (m *mockKeyRepo) GetByAddress(ctx context.Context, orgID uuid.UUID, address string) (*models.Key, error) {
	for _, key := range m.keys {
		if key.OrgID == orgID && key.Address == address && key.DeletedAt == nil {
			return key, nil
		}
	}
	return nil, nil
}

func (m *mockKeyRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]*models.Key, error) {
	var result []*models.Key
	for _, key := range m.keys {
		if key.OrgID == orgID && key.DeletedAt == nil {
			result = append(result, key)
		}
	}
	return result, nil
}

func (m *mockKeyRepo) ListByNamespace(ctx context.Context, namespaceID uuid.UUID) ([]*models.Key, error) {
	var result []*models.Key
	for _, key := range m.keys {
		if key.NamespaceID == namespaceID && key.DeletedAt == nil {
			result = append(result, key)
		}
	}
	return result, nil
}

func (m *mockKeyRepo) CountByOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	count := 0
	for _, key := range m.keys {
		if key.OrgID == orgID && key.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (m *mockKeyRepo) Update(ctx context.Context, key *models.Key) error {
	m.keys[key.ID] = key
	return nil
}

func (m *mockKeyRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if key, ok := m.keys[id]; ok {
		now := time.Now()
		key.DeletedAt = &now
	}
	return nil
}

func (m *mockKeyRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.keys, id)
	return nil
}

type mockOrgRepo struct {
	orgs        map[uuid.UUID]*models.Organization
	namespaces  map[uuid.UUID]*models.Namespace
	members     map[string]*models.OrgMember // orgID_userID -> member
	invitations map[uuid.UUID]*models.Invitation
}

func newMockOrgRepo() *mockOrgRepo {
	return &mockOrgRepo{
		orgs:        make(map[uuid.UUID]*models.Organization),
		namespaces:  make(map[uuid.UUID]*models.Namespace),
		members:     make(map[string]*models.OrgMember),
		invitations: make(map[uuid.UUID]*models.Invitation),
	}
}

func (m *mockOrgRepo) Create(ctx context.Context, org *models.Organization, ownerID uuid.UUID) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	org.CreatedAt = time.Now()
	org.UpdatedAt = org.CreatedAt
	m.orgs[org.ID] = org
	// Add owner as member
	key := org.ID.String() + "_" + ownerID.String()
	m.members[key] = &models.OrgMember{
		OrgID:    org.ID,
		UserID:   ownerID,
		Role:     models.RoleOwner,
		JoinedAt: time.Now(),
	}
	return nil
}

func (m *mockOrgRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Organization, error) {
	return m.orgs[id], nil
}

func (m *mockOrgRepo) GetBySlug(ctx context.Context, slug string) (*models.Organization, error) {
	for _, org := range m.orgs {
		if org.Slug == slug {
			return org, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) Update(ctx context.Context, org *models.Organization) error {
	m.orgs[org.ID] = org
	return nil
}

func (m *mockOrgRepo) Delete(ctx context.Context, id uuid.UUID) error {
	delete(m.orgs, id)
	return nil
}

func (m *mockOrgRepo) AddMember(ctx context.Context, orgID, userID uuid.UUID, role models.Role, invitedBy *uuid.UUID) error {
	key := orgID.String() + "_" + userID.String()
	m.members[key] = &models.OrgMember{
		OrgID:     orgID,
		UserID:    userID,
		Role:      role,
		InvitedBy: invitedBy,
		JoinedAt:  time.Now(),
	}
	return nil
}

func (m *mockOrgRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	key := orgID.String() + "_" + userID.String()
	delete(m.members, key)
	return nil
}

func (m *mockOrgRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role models.Role) error {
	key := orgID.String() + "_" + userID.String()
	if member, ok := m.members[key]; ok {
		member.Role = role
	}
	return nil
}

func (m *mockOrgRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*models.OrgMember, error) {
	key := orgID.String() + "_" + userID.String()
	return m.members[key], nil
}

func (m *mockOrgRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*models.OrgMember, error) {
	var result []*models.OrgMember
	for key, member := range m.members {
		if len(key) >= 36 && key[:36] == orgID.String() {
			result = append(result, member)
		}
	}
	return result, nil
}

func (m *mockOrgRepo) ListUserOrgs(ctx context.Context, userID uuid.UUID) ([]*models.Organization, error) {
	var result []*models.Organization
	for _, member := range m.members {
		if member.UserID == userID {
			if org, ok := m.orgs[member.OrgID]; ok {
				result = append(result, org)
			}
		}
	}
	return result, nil
}

func (m *mockOrgRepo) CountMembers(ctx context.Context, orgID uuid.UUID) (int, error) {
	count := 0
	for key := range m.members {
		if len(key) >= 36 && key[:36] == orgID.String() {
			count++
		}
	}
	return count, nil
}

func (m *mockOrgRepo) CreateNamespace(ctx context.Context, ns *models.Namespace) error {
	if ns.ID == uuid.Nil {
		ns.ID = uuid.New()
	}
	ns.CreatedAt = time.Now()
	m.namespaces[ns.ID] = ns
	return nil
}

func (m *mockOrgRepo) GetNamespace(ctx context.Context, id uuid.UUID) (*models.Namespace, error) {
	return m.namespaces[id], nil
}

func (m *mockOrgRepo) GetNamespaceByName(ctx context.Context, orgID uuid.UUID, name string) (*models.Namespace, error) {
	for _, ns := range m.namespaces {
		if ns.OrgID == orgID && ns.Name == name {
			return ns, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) ListNamespaces(ctx context.Context, orgID uuid.UUID) ([]*models.Namespace, error) {
	var result []*models.Namespace
	for _, ns := range m.namespaces {
		if ns.OrgID == orgID {
			result = append(result, ns)
		}
	}
	return result, nil
}

func (m *mockOrgRepo) DeleteNamespace(ctx context.Context, id uuid.UUID) error {
	delete(m.namespaces, id)
	return nil
}

func (m *mockOrgRepo) CountNamespaces(ctx context.Context, orgID uuid.UUID) (int, error) {
	count := 0
	for _, ns := range m.namespaces {
		if ns.OrgID == orgID {
			count++
		}
	}
	return count, nil
}

func (m *mockOrgRepo) CreateInvitation(ctx context.Context, inv *models.Invitation) error {
	if inv.ID == uuid.Nil {
		inv.ID = uuid.New()
	}
	inv.CreatedAt = time.Now()
	m.invitations[inv.ID] = inv
	return nil
}

func (m *mockOrgRepo) GetInvitationByToken(ctx context.Context, token string) (*models.Invitation, error) {
	for _, inv := range m.invitations {
		if inv.Token == token {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) GetInvitationByEmail(ctx context.Context, orgID uuid.UUID, email string) (*models.Invitation, error) {
	for _, inv := range m.invitations {
		if inv.OrgID == orgID && inv.Email == email {
			return inv, nil
		}
	}
	return nil, nil
}

func (m *mockOrgRepo) ListPendingInvitations(ctx context.Context, orgID uuid.UUID) ([]*models.Invitation, error) {
	var result []*models.Invitation
	for _, inv := range m.invitations {
		if inv.OrgID == orgID && inv.AcceptedAt == nil {
			result = append(result, inv)
		}
	}
	return result, nil
}

func (m *mockOrgRepo) AcceptInvitation(ctx context.Context, token string, userID uuid.UUID) error {
	for _, inv := range m.invitations {
		if inv.Token == token {
			now := time.Now()
			inv.AcceptedAt = &now
			// Add as member
			return m.AddMember(ctx, inv.OrgID, userID, inv.Role, &inv.InvitedBy)
		}
	}
	return nil
}

func (m *mockOrgRepo) DeleteInvitation(ctx context.Context, id uuid.UUID) error {
	delete(m.invitations, id)
	return nil
}

type mockAuditRepo struct {
	logs []*models.AuditLog
}

func newMockAuditRepo() *mockAuditRepo {
	return &mockAuditRepo{logs: make([]*models.AuditLog, 0)}
}

func (m *mockAuditRepo) Create(ctx context.Context, log *models.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.CreatedAt = time.Now()
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockAuditRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	for _, log := range m.logs {
		if log.ID == id {
			return log, nil
		}
	}
	return nil, nil
}

func (m *mockAuditRepo) List(ctx context.Context, query models.AuditLogQuery) ([]*models.AuditLog, error) {
	var result []*models.AuditLog
	for _, log := range m.logs {
		if log.OrgID == query.OrgID {
			result = append(result, log)
		}
	}
	return result, nil
}

func (m *mockAuditRepo) DeleteBefore(ctx context.Context, orgID uuid.UUID, before time.Time) (int64, error) {
	return 0, nil
}

type mockUsageRepo struct {
	usage map[string]int64 // orgID_metric -> value
}

func newMockUsageRepo() *mockUsageRepo {
	return &mockUsageRepo{usage: make(map[string]int64)}
}

func (m *mockUsageRepo) Increment(ctx context.Context, orgID uuid.UUID, metric string, value int64) error {
	key := orgID.String() + "_" + metric
	m.usage[key] += value
	return nil
}

func (m *mockUsageRepo) GetCurrentPeriod(ctx context.Context, orgID uuid.UUID, metric string) (int64, error) {
	key := orgID.String() + "_" + metric
	return m.usage[key], nil
}

func (m *mockUsageRepo) GetMetric(ctx context.Context, orgID uuid.UUID, metric string, periodStart time.Time) (*models.UsageMetric, error) {
	return nil, nil
}

func (m *mockUsageRepo) ListByOrg(ctx context.Context, orgID uuid.UUID, periodStart, periodEnd time.Time) ([]*models.UsageMetric, error) {
	return nil, nil
}

func (m *mockUsageRepo) GetSummary(ctx context.Context, orgID uuid.UUID, plan models.Plan) (*models.UsageSummary, error) {
	return nil, nil
}

// --- Mock BaoKeyring ---

type mockBaoKeyring struct {
	keys      map[string]*mockBaoKey
	signCount int
}

type mockBaoKey struct {
	name       string
	pubKey     []byte
	address    string
	exportable bool
	privateKey string
}

func newMockBaoKeyring() *mockBaoKeyring {
	return &mockBaoKeyring{keys: make(map[string]*mockBaoKey)}
}

func (m *mockBaoKeyring) NewAccountWithOptions(uid string, opts KeyOptions) ([]byte, string, error) {
	// Generate a mock 33-byte compressed secp256k1 public key
	pubKey := make([]byte, 33)
	pubKey[0] = 0x02 // compressed format prefix
	for i := 1; i < 33; i++ {
		pubKey[i] = byte(i)
	}

	nameLen := len(uid)
	if nameLen > 39 {
		nameLen = 39
	}
	address := "celestia1" + uid[:nameLen]

	key := &mockBaoKey{
		name:       uid,
		pubKey:     pubKey,
		address:    address,
		exportable: opts.Exportable,
		privateKey: base64.StdEncoding.EncodeToString([]byte("mock_private_key_" + uid)),
	}
	m.keys[uid] = key

	return pubKey, address, nil
}

func (m *mockBaoKeyring) Sign(uid string, msg []byte) ([]byte, []byte, error) {
	key, ok := m.keys[uid]
	if !ok {
		return nil, nil, apierrors.NewNotFoundError("Key")
	}

	// Mock 64-byte signature
	sig := make([]byte, 64)
	for i := range sig {
		sig[i] = byte(i)
	}
	m.signCount++

	return sig, key.pubKey, nil
}

func (m *mockBaoKeyring) Delete(uid string) error {
	delete(m.keys, uid)
	return nil
}

func (m *mockBaoKeyring) GetMetadata(uid string) (*KeyMetadata, error) {
	key, ok := m.keys[uid]
	if !ok {
		return nil, apierrors.NewNotFoundError("Key")
	}
	return &KeyMetadata{
		UID:         uid,
		Name:        uid,
		PubKeyBytes: key.pubKey,
		Address:     key.address,
	}, nil
}

func (m *mockBaoKeyring) ImportKey(uid string, ciphertext string, exportable bool) ([]byte, string, error) {
	// Generate a mock public key
	pubKey := make([]byte, 33)
	pubKey[0] = 0x02
	for i := 1; i < 33; i++ {
		pubKey[i] = byte(i + 10) // Different from generated keys
	}

	nameLen := len(uid)
	if nameLen > 39 {
		nameLen = 39
	}
	address := "celestia1" + uid[:nameLen]

	key := &mockBaoKey{
		name:       uid,
		pubKey:     pubKey,
		address:    address,
		exportable: exportable,
		privateKey: ciphertext,
	}
	m.keys[uid] = key

	return pubKey, address, nil
}

func (m *mockBaoKeyring) ExportKey(uid string) (string, error) {
	key, ok := m.keys[uid]
	if !ok {
		return "", apierrors.NewNotFoundError("Key")
	}
	if !key.exportable {
		return "", apierrors.ErrForbidden.WithMessage("Key is not exportable")
	}
	return key.privateKey, nil
}

// --- Test Key Service ---

type testKeyService struct {
	keyRepo    *mockKeyRepo
	orgRepo    *mockOrgRepo
	auditRepo  *mockAuditRepo
	usageRepo  *mockUsageRepo
	baoKeyring *mockBaoKeyring
	svc        KeyService
}

func newTestKeyService() *testKeyService {
	keyRepo := newMockKeyRepo()
	orgRepo := newMockOrgRepo()
	auditRepo := newMockAuditRepo()
	usageRepo := newMockUsageRepo()
	baoKeyring := newMockBaoKeyring()

	svc := NewKeyService(keyRepo, orgRepo, auditRepo, usageRepo, baoKeyring)

	return &testKeyService{
		keyRepo:    keyRepo,
		orgRepo:    orgRepo,
		auditRepo:  auditRepo,
		usageRepo:  usageRepo,
		baoKeyring: baoKeyring,
		svc:        svc,
	}
}

// Create a test org and namespace
func (s *testKeyService) createTestOrgAndNamespace(plan models.Plan) (orgID, namespaceID uuid.UUID) {
	orgID = uuid.New()
	namespaceID = uuid.New()

	s.orgRepo.orgs[orgID] = &models.Organization{
		ID:        orgID,
		Name:      "Test Org",
		Slug:      "test-org",
		Plan:      plan,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.orgRepo.namespaces[namespaceID] = &models.Namespace{
		ID:        namespaceID,
		OrgID:     orgID,
		Name:      "default",
		CreatedAt: time.Now(),
	}

	return orgID, namespaceID
}

// --- Tests ---

func TestKeyService_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("creates key successfully", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, err := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "test-key",
			Exportable:  false,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if key == nil {
			t.Fatal("Create() returned nil key")
		}

		if key.Name != "test-key" {
			t.Errorf("Name = %v, want %v", key.Name, "test-key")
		}

		if key.OrgID != orgID {
			t.Errorf("OrgID = %v, want %v", key.OrgID, orgID)
		}

		if key.NamespaceID != nsID {
			t.Errorf("NamespaceID = %v, want %v", key.NamespaceID, nsID)
		}

		if key.Algorithm != models.AlgorithmSecp256k1 {
			t.Errorf("Algorithm = %v, want %v", key.Algorithm, models.AlgorithmSecp256k1)
		}

		if len(key.PublicKey) != 33 {
			t.Errorf("PublicKey length = %v, want 33", len(key.PublicKey))
		}
	})

	t.Run("creates exportable key", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, err := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "exportable-key",
			Exportable:  true,
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if !key.Exportable {
			t.Error("Exportable = false, want true")
		}
	})

	t.Run("enforces key quota", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanFree) // 3 key limit

		// Create keys up to limit
		for i := 0; i < 3; i++ {
			_, err := ts.svc.Create(ctx, CreateKeyRequest{
				OrgID:       orgID,
				NamespaceID: nsID,
				Name:        "key-" + string(rune('a'+i)),
			})
			if err != nil {
				t.Fatalf("Create() error = %v", err)
			}
		}

		// Try to create one more
		_, err := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "over-limit",
		})
		if err == nil {
			t.Error("Create() expected quota error")
		}
	})

	t.Run("enterprise has unlimited keys", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanEnterprise)

		// Create many keys
		for i := 0; i < 10; i++ {
			_, err := ts.svc.Create(ctx, CreateKeyRequest{
				OrgID:       orgID,
				NamespaceID: nsID,
				Name:        "key-" + string(rune('a'+i)),
			})
			if err != nil {
				t.Fatalf("Create() error on key %d = %v", i, err)
			}
		}
	})

	t.Run("rejects invalid namespace", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, _ := ts.createTestOrgAndNamespace(models.PlanPro)

		_, err := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: uuid.New(), // Non-existent namespace
			Name:        "test-key",
		})
		if err == nil {
			t.Error("Create() expected error for invalid namespace")
		}
	})
}

func TestKeyService_Get(t *testing.T) {
	ctx := context.Background()

	t.Run("gets key successfully", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		created, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "test-key",
		})

		key, err := ts.svc.Get(ctx, orgID, created.ID)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if key.ID != created.ID {
			t.Errorf("ID = %v, want %v", key.ID, created.ID)
		}
	})

	t.Run("returns 404 for wrong org", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)
		otherOrgID, _ := ts.createTestOrgAndNamespace(models.PlanPro)

		created, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "test-key",
		})

		_, err := ts.svc.Get(ctx, otherOrgID, created.ID)
		if err == nil {
			t.Error("Get() expected error for wrong org")
		}
	})
}

func TestKeyService_List(t *testing.T) {
	ctx := context.Background()

	t.Run("lists keys by org", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		// Create some keys
		for i := 0; i < 3; i++ {
			ts.svc.Create(ctx, CreateKeyRequest{
				OrgID:       orgID,
				NamespaceID: nsID,
				Name:        "key-" + string(rune('a'+i)),
			})
		}

		keys, err := ts.svc.List(ctx, orgID, nil)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(keys) != 3 {
			t.Errorf("List() returned %d keys, want 3", len(keys))
		}
	})

	t.Run("lists keys by namespace", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		// Create another namespace
		nsID2 := uuid.New()
		ts.orgRepo.namespaces[nsID2] = &models.Namespace{
			ID:        nsID2,
			OrgID:     orgID,
			Name:      "production",
			CreatedAt: time.Now(),
		}

		// Create keys in different namespaces
		ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "dev-key",
		})
		ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID2,
			Name:        "prod-key",
		})

		keys, err := ts.svc.List(ctx, orgID, &nsID)
		if err != nil {
			t.Fatalf("List() error = %v", err)
		}

		if len(keys) != 1 {
			t.Errorf("List() returned %d keys, want 1", len(keys))
		}
	})
}

func TestKeyService_Sign(t *testing.T) {
	ctx := context.Background()

	t.Run("signs data successfully", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "sign-key",
		})

		data := []byte("hello world")
		resp, err := ts.svc.Sign(ctx, orgID, key.ID, data, false)
		if err != nil {
			t.Fatalf("Sign() error = %v", err)
		}

		if resp.KeyID != key.ID {
			t.Errorf("KeyID = %v, want %v", resp.KeyID, key.ID)
		}

		if resp.Signature == "" {
			t.Error("Signature is empty")
		}

		if resp.PublicKey == "" {
			t.Error("PublicKey is empty")
		}
	})

	t.Run("enforces signature quota", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanFree) // 10000 sig limit

		key, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "sign-key",
		})

		// Simulate usage at limit
		ts.usageRepo.usage[orgID.String()+"_signatures"] = 10000

		data := []byte("hello world")
		_, err := ts.svc.Sign(ctx, orgID, key.ID, data, false)
		if err == nil {
			t.Error("Sign() expected quota error")
		}
	})

	t.Run("rejects sign for wrong org", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)
		otherOrgID, _ := ts.createTestOrgAndNamespace(models.PlanPro)

		key, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "sign-key",
		})

		data := []byte("hello world")
		_, err := ts.svc.Sign(ctx, otherOrgID, key.ID, data, false)
		if err == nil {
			t.Error("Sign() expected error for wrong org")
		}
	})
}

func TestKeyService_Delete(t *testing.T) {
	ctx := context.Background()

	t.Run("soft deletes key", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "delete-key",
		})

		err := ts.svc.Delete(ctx, orgID, key.ID)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Key should still exist in DB but be marked deleted
		deletedKey, _ := ts.keyRepo.GetByID(ctx, key.ID)
		if deletedKey == nil {
			t.Error("Key not found after soft delete")
		}
		if deletedKey.DeletedAt == nil {
			t.Error("DeletedAt is nil after soft delete")
		}

		// Should not appear in list
		keys, _ := ts.svc.List(ctx, orgID, nil)
		if len(keys) != 0 {
			t.Errorf("List() returned %d keys after delete, want 0", len(keys))
		}

		// Get should return error
		_, err = ts.svc.Get(ctx, orgID, key.ID)
		if err == nil {
			t.Error("Get() expected error for deleted key")
		}
	})
}

func TestKeyService_BatchCreate(t *testing.T) {
	ctx := context.Background()

	t.Run("batch creates keys", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanEnterprise)

		keys, err := ts.svc.CreateBatch(ctx, CreateBatchKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Prefix:      "worker",
			Count:       4,
		})
		if err != nil {
			t.Fatalf("CreateBatch() error = %v", err)
		}

		if len(keys) != 4 {
			t.Errorf("CreateBatch() created %d keys, want 4", len(keys))
		}

		// Verify names
		for i, key := range keys {
			expectedName := "worker-" + string(rune('1'+i))
			if key.Name != expectedName {
				t.Errorf("Key %d name = %v, want %v", i, key.Name, expectedName)
			}
		}
	})

	t.Run("enforces quota in batch", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanFree) // 3 key limit

		_, err := ts.svc.CreateBatch(ctx, CreateBatchKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Prefix:      "worker",
			Count:       5,
		})
		if err == nil {
			t.Error("CreateBatch() expected quota error")
		}
	})
}

func TestKeyService_SignBatch(t *testing.T) {
	ctx := context.Background()

	t.Run("batch signs successfully", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		// Create keys
		keys, _ := ts.svc.CreateBatch(ctx, CreateBatchKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Prefix:      "sign-worker",
			Count:       3,
		})

		// Sign with all keys
		requests := make([]SignKeyRequest, len(keys))
		for i, key := range keys {
			requests[i] = SignKeyRequest{
				KeyID: key.ID,
				Data:  base64.StdEncoding.EncodeToString([]byte("test data " + string(rune('a'+i)))),
			}
		}

		results, err := ts.svc.SignBatch(ctx, SignBatchKeyRequest{
			OrgID:    orgID,
			Requests: requests,
		})
		if err != nil {
			t.Fatalf("SignBatch() error = %v", err)
		}

		if len(results) != 3 {
			t.Errorf("SignBatch() returned %d results, want 3", len(results))
		}

		for i, result := range results {
			if result.Error != "" {
				t.Errorf("Result %d has error: %s", i, result.Error)
			}
			if result.Signature == "" {
				t.Errorf("Result %d has empty signature", i)
			}
		}
	})
}

func TestKeyService_ImportExport(t *testing.T) {
	ctx := context.Background()

	t.Run("imports and exports key", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		privateKey := base64.StdEncoding.EncodeToString([]byte("test_private_key"))

		key, err := ts.svc.Import(ctx, ImportKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "imported-key",
			PrivateKey:  privateKey,
			Exportable:  true,
		})
		if err != nil {
			t.Fatalf("Import() error = %v", err)
		}

		if key.Name != "imported-key" {
			t.Errorf("Name = %v, want 'imported-key'", key.Name)
		}

		// Export the key
		exported, err := ts.svc.Export(ctx, orgID, key.ID)
		if err != nil {
			t.Fatalf("Export() error = %v", err)
		}

		if exported == "" {
			t.Error("Export() returned empty string")
		}
	})

	t.Run("rejects export of non-exportable key", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, _ := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "non-exportable",
			Exportable:  false,
		})

		_, err := ts.svc.Export(ctx, orgID, key.ID)
		if err == nil {
			t.Error("Export() expected error for non-exportable key")
		}
	})
}

func TestKeyService_Metadata(t *testing.T) {
	ctx := context.Background()

	t.Run("stores metadata correctly", func(t *testing.T) {
		ts := newTestKeyService()
		orgID, nsID := ts.createTestOrgAndNamespace(models.PlanPro)

		key, err := ts.svc.Create(ctx, CreateKeyRequest{
			OrgID:       orgID,
			NamespaceID: nsID,
			Name:        "metadata-key",
			Metadata: map[string]string{
				"purpose": "signing",
				"env":     "production",
			},
		})
		if err != nil {
			t.Fatalf("Create() error = %v", err)
		}

		if key.Metadata == nil {
			t.Error("Metadata is nil")
		}

		var parsedMeta map[string]string
		json.Unmarshal(key.Metadata, &parsedMeta)

		if parsedMeta["purpose"] != "signing" {
			t.Errorf("Metadata purpose = %v, want 'signing'", parsedMeta["purpose"])
		}
		if parsedMeta["env"] != "production" {
			t.Errorf("Metadata env = %v, want 'production'", parsedMeta["env"])
		}
	})
}
