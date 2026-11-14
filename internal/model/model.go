// Package model holds the table models for the core schema (issue #2).
// JSONB columns are carried as json.RawMessage and (un)marshalled by the
// repositories.
package model

import (
	"encoding/json"
	"time"
)

// Tenant roles (issue #6). The workflow-relative "creator" role is resolved
// at runtime against the ticket creator and is intentionally not a tenant role.
const (
	RoleTenantAdmin = "tenant_admin"
	RoleAgent       = "agent"
	RoleRequester   = "requester"
)

// Tenant is a customer organization.
type Tenant struct {
	ID        int64
	Name      string
	Slug      string
	LogoURL   string
	Theme     json.RawMessage
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// User is a global account; one email may join multiple tenants.
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	DisplayName  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TenantMember links a user into a tenant with a role.
type TenantMember struct {
	ID        int64
	TenantID  int64
	UserID    int64
	Role      string
	CreatedAt time.Time
}

// TicketType is the template pack: form schema, workflow, SLA, assignment
// and view config, all JSONB validated at save time (issue #4).
type TicketType struct {
	ID          int64
	TenantID    int64
	Slug        string
	Name        string
	Icon        string
	FormSchema  json.RawMessage
	Workflow    json.RawMessage
	SLAPolicy   json.RawMessage
	Assignment  json.RawMessage
	ViewConfig  json.RawMessage
	IsBuiltin   bool
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Ticket priorities.
const (
	PriorityLow    = "low"
	PriorityNormal = "normal"
	PriorityHigh   = "high"
	PriorityUrgent = "urgent"
)

// Ticket is a single work item. State holds a workflow state key; values are
// validated by the state machine, not by a database enum.
type Ticket struct {
	ID           int64
	TenantID     int64
	TicketTypeID int64
	Number       string
	Title        string
	Content      string
	Priority     string
	State        string
	CustomFields json.RawMessage
	CreatorID    int64
	AssigneeID   *int64
	DueAt        *time.Time
	SLADeadlines json.RawMessage
	ClosedAt     *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Timeline kinds (single source of truth per issue #2).
const (
	KindComment      = "comment"
	KindInternalNote = "internal_note"
	KindStateChange  = "state_change"
	KindAssignment   = "assignment"
	KindSystem       = "system"
)

// TicketTimeline records every conversation entry, internal note, state
// change, assignment and system event of a ticket.
type TicketTimeline struct {
	ID        int64
	TenantID  int64
	TicketID  int64
	Kind      string
	AuthorID  int64
	Body      string
	Meta      json.RawMessage
	CreatedAt time.Time
}

// Attachment is an uploaded file bound to a ticket.
type Attachment struct {
	ID          int64
	TenantID    int64
	TicketID    int64
	TimelineID  *int64
	Filename    string
	SizeBytes   int64
	ContentType string
	StoragePath string
	SHA256      string
	UploaderID  int64
	CreatedAt   time.Time
}

// Notification is an in-app notification row.
type Notification struct {
	ID        int64
	TenantID  int64
	UserID    int64
	TicketID  *int64
	Kind      string
	Title     string
	Body      string
	ReadAt    *time.Time
	CreatedAt time.Time
}

// Ticket statuses for tenants and ticket types.
const (
	StatusActive  = "active"
	StatusEnabled = "enabled"
	StatusDisabled = "disabled"
)
