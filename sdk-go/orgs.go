package banhbaoring

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// OrgsService handles organization operations.
type OrgsService struct {
	client *Client
}

// CreateOrgRequest is the request for creating an organization.
type CreateOrgRequest struct {
	// Name is the organization name (required).
	Name string `json:"name"`
}

// UpdateOrgRequest is the request for updating an organization.
type UpdateOrgRequest struct {
	// Name is the new organization name.
	Name string `json:"name,omitempty"`
}

// InviteMemberRequest is the request for inviting a member.
type InviteMemberRequest struct {
	// Email is the email address to invite.
	Email string `json:"email"`
	// Role is the role to assign to the member.
	Role Role `json:"role"`
}

// UpdateMemberRoleRequest is the request for updating a member's role.
type UpdateMemberRoleRequest struct {
	// Role is the new role.
	Role Role `json:"role"`
}

// CreateNamespaceRequest is the request for creating a namespace.
type CreateNamespaceRequest struct {
	// Name is the namespace name (required).
	Name string `json:"name"`
	// Description is an optional description.
	Description string `json:"description,omitempty"`
}

// Create creates a new organization.
//
// Example:
//
//	org, err := client.Orgs.Create(ctx, banhbaoring.CreateOrgRequest{
//	    Name: "My Organization",
//	})
func (s *OrgsService) Create(ctx context.Context, req CreateOrgRequest) (*Organization, error) {
	var resp Organization
	if err := s.client.post(ctx, "/v1/organizations", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// List returns all organizations the current user has access to.
//
// Example:
//
//	orgs, err := client.Orgs.List(ctx)
func (s *OrgsService) List(ctx context.Context) ([]*Organization, error) {
	var resp []*Organization
	if err := s.client.get(ctx, "/v1/organizations", &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Get retrieves an organization by ID.
//
// Example:
//
//	org, err := client.Orgs.Get(ctx, orgID)
func (s *OrgsService) Get(ctx context.Context, orgID uuid.UUID) (*Organization, error) {
	var resp Organization
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s", orgID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Update updates an organization.
//
// Example:
//
//	org, err := client.Orgs.Update(ctx, orgID, banhbaoring.UpdateOrgRequest{
//	    Name: "New Name",
//	})
func (s *OrgsService) Update(ctx context.Context, orgID uuid.UUID, req UpdateOrgRequest) (*Organization, error) {
	var resp Organization
	if err := s.client.patch(ctx, fmt.Sprintf("/v1/organizations/%s", orgID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Delete deletes an organization.
//
// Example:
//
//	err := client.Orgs.Delete(ctx, orgID)
func (s *OrgsService) Delete(ctx context.Context, orgID uuid.UUID) error {
	return s.client.delete(ctx, fmt.Sprintf("/v1/organizations/%s", orgID))
}

// GetLimits retrieves the plan limits for an organization.
//
// Example:
//
//	limits, err := client.Orgs.GetLimits(ctx, orgID)
//	fmt.Printf("Keys: %d/%d\n", limits.CurrentKeys, limits.MaxKeys)
func (s *OrgsService) GetLimits(ctx context.Context, orgID uuid.UUID) (*PlanLimits, error) {
	var resp PlanLimits
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/limits", orgID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ListMembers returns all members of an organization.
//
// Example:
//
//	members, err := client.Orgs.ListMembers(ctx, orgID)
func (s *OrgsService) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*Member, error) {
	var resp []*Member
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/members", orgID), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InviteMember invites a user to join an organization.
//
// Example:
//
//	invitation, err := client.Orgs.InviteMember(ctx, orgID, banhbaoring.InviteMemberRequest{
//	    Email: "user@example.com",
//	    Role:  banhbaoring.RoleOperator,
//	})
func (s *OrgsService) InviteMember(ctx context.Context, orgID uuid.UUID, req InviteMemberRequest) (*Invitation, error) {
	var resp Invitation
	if err := s.client.post(ctx, fmt.Sprintf("/v1/organizations/%s/members", orgID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveMember removes a member from an organization.
//
// Example:
//
//	err := client.Orgs.RemoveMember(ctx, orgID, userID)
func (s *OrgsService) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.client.delete(ctx, fmt.Sprintf("/v1/organizations/%s/members/%s", orgID, userID))
}

// UpdateMemberRole updates a member's role.
//
// Example:
//
//	err := client.Orgs.UpdateMemberRole(ctx, orgID, userID, banhbaoring.UpdateMemberRoleRequest{
//	    Role: banhbaoring.RoleAdmin,
//	})
func (s *OrgsService) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, req UpdateMemberRoleRequest) error {
	return s.client.patch(ctx, fmt.Sprintf("/v1/organizations/%s/members/%s", orgID, userID), req, nil)
}

// ListInvitations returns all pending invitations for an organization.
//
// Example:
//
//	invitations, err := client.Orgs.ListInvitations(ctx, orgID)
func (s *OrgsService) ListInvitations(ctx context.Context, orgID uuid.UUID) ([]*Invitation, error) {
	var resp []*Invitation
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/invitations", orgID), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CancelInvitation cancels a pending invitation.
//
// Example:
//
//	err := client.Orgs.CancelInvitation(ctx, orgID, invitationID)
func (s *OrgsService) CancelInvitation(ctx context.Context, orgID, invitationID uuid.UUID) error {
	return s.client.delete(ctx, fmt.Sprintf("/v1/organizations/%s/invitations/%s", orgID, invitationID))
}

// ListNamespaces returns all namespaces in an organization.
//
// Example:
//
//	namespaces, err := client.Orgs.ListNamespaces(ctx, orgID)
func (s *OrgsService) ListNamespaces(ctx context.Context, orgID uuid.UUID) ([]*Namespace, error) {
	var resp []*Namespace
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/namespaces", orgID), &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CreateNamespace creates a new namespace.
//
// Example:
//
//	ns, err := client.Orgs.CreateNamespace(ctx, orgID, banhbaoring.CreateNamespaceRequest{
//	    Name:        "production",
//	    Description: "Production keys",
//	})
func (s *OrgsService) CreateNamespace(ctx context.Context, orgID uuid.UUID, req CreateNamespaceRequest) (*Namespace, error) {
	var resp Namespace
	if err := s.client.post(ctx, fmt.Sprintf("/v1/organizations/%s/namespaces", orgID), req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetNamespace retrieves a namespace by ID.
//
// Example:
//
//	ns, err := client.Orgs.GetNamespace(ctx, orgID, namespaceID)
func (s *OrgsService) GetNamespace(ctx context.Context, orgID, namespaceID uuid.UUID) (*Namespace, error) {
	var resp Namespace
	if err := s.client.get(ctx, fmt.Sprintf("/v1/organizations/%s/namespaces/%s", orgID, namespaceID), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DeleteNamespace deletes a namespace.
//
// Example:
//
//	err := client.Orgs.DeleteNamespace(ctx, orgID, namespaceID)
func (s *OrgsService) DeleteNamespace(ctx context.Context, orgID, namespaceID uuid.UUID) error {
	return s.client.delete(ctx, fmt.Sprintf("/v1/organizations/%s/namespaces/%s", orgID, namespaceID))
}

