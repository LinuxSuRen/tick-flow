package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/templating"
)

// TicketTypeService validates and persists template packs (issue #12).
type TicketTypeService struct {
	DB *sql.DB
}

// TypeView is the API projection of a ticket type: identity plus the five
// decoded JSONB sections.
type TypeView struct {
	ID         int64                     `json:"id"`
	Slug       string                    `json:"slug"`
	Name       string                    `json:"name"`
	Icon       string                    `json:"icon"`
	FormSchema json.RawMessage           `json:"form_schema"`
	Workflow   json.RawMessage           `json:"workflow"`
	SLAPolicy  json.RawMessage           `json:"sla_policy"`
	Assignment json.RawMessage           `json:"assignment"`
	ViewConfig json.RawMessage           `json:"view_config"`
	IsBuiltin  bool                      `json:"is_builtin"`
	Status     string                    `json:"status"`
	CreatedAt  string                    `json:"created_at"`
	UpdatedAt  string                    `json:"updated_at"`
}

func toTypeView(t *model.TicketType) *TypeView {
	return &TypeView{
		ID: t.ID, Slug: t.Slug, Name: t.Name, Icon: t.Icon,
		FormSchema: jsonOrDefault(t.FormSchema), Workflow: jsonOrDefault(t.Workflow),
		SLAPolicy: jsonOrDefault(t.SLAPolicy), Assignment: jsonOrDefault(t.Assignment),
		ViewConfig: jsonOrDefault(t.ViewConfig),
		IsBuiltin: t.IsBuiltin, Status: t.Status,
		CreatedAt: t.CreatedAt.UTC().Format(timeFormat), UpdatedAt: t.UpdatedAt.UTC().Format(timeFormat),
	}
}

const timeFormat = "2006-01-02T15:04:05Z"

func jsonOrDefault(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("{}")
	}
	return raw
}

// Create validates and inserts a template pack.
func (s *TicketTypeService) Create(ctx context.Context, payload []byte) (*TypeView, error) {
	pack, err := templating.ParsePackJSON(payload)
	if err != nil {
		return nil, repository.ErrValidation("malformed ticket type payload: " + err.Error())
	}
	pack.Slug = templating.NormalizeSlug(pack.Slug)
	if ve := pack.Validate(); !ve.Empty() {
		return nil, ve
	}
	repo := &repository.TicketTypes{Q: s.DB}
	id, err := repo.Create(ctx, &model.TicketType{
		Slug: pack.Slug, Name: pack.Name, Icon: pack.Icon,
		FormSchema: pack.FormSchemaJSON(), Workflow: pack.WorkflowJSON(),
		SLAPolicy: pack.SLAPolicyJSON(), Assignment: pack.AssignmentJSON(),
		ViewConfig: pack.ViewConfigJSON(),
	})
	if err != nil {
		return nil, err
	}
	created, err := repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toTypeView(created), nil
}

// Update validates and rewrites a template pack.
func (s *TicketTypeService) Update(ctx context.Context, id int64, payload []byte) (*TypeView, error) {
	pack, err := templating.ParsePackJSON(payload)
	if err != nil {
		return nil, repository.ErrValidation("malformed ticket type payload: " + err.Error())
	}
	pack.Slug = templating.NormalizeSlug(pack.Slug)
	if ve := pack.Validate(); !ve.Empty() {
		return nil, ve
	}
	repo := &repository.TicketTypes{Q: s.DB}
	if err := repo.Update(ctx, id, &model.TicketType{
		Slug: pack.Slug, Name: pack.Name, Icon: pack.Icon,
		FormSchema: pack.FormSchemaJSON(), Workflow: pack.WorkflowJSON(),
		SLAPolicy: pack.SLAPolicyJSON(), Assignment: pack.AssignmentJSON(),
		ViewConfig: pack.ViewConfigJSON(),
	}); err != nil {
		return nil, err
	}
	updated, err := repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toTypeView(updated), nil
}

// Get loads one type.
func (s *TicketTypeService) Get(ctx context.Context, id int64) (*TypeView, error) {
	t, err := (&repository.TicketTypes{Q: s.DB}).ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toTypeView(t), nil
}

// ListEnabled lists portal-visible types.
func (s *TicketTypeService) ListEnabled(ctx context.Context) ([]*TypeView, error) {
	items, err := (&repository.TicketTypes{Q: s.DB}).ListEnabled(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*TypeView, 0, len(items))
	for _, t := range items {
		out = append(out, toTypeView(t))
	}
	return out, nil
}

// ListAll pages every type for the admin view.
func (s *TicketTypeService) ListAll(ctx context.Context, page, pageSize int) ([]*TypeView, int64, error) {
	items, total, err := (&repository.TicketTypes{Q: s.DB}).ListAll(ctx, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*TypeView, 0, len(items))
	for _, t := range items {
		out = append(out, toTypeView(t))
	}
	return out, total, nil
}

// SetStatus enables or disables a type.
func (s *TicketTypeService) SetStatus(ctx context.Context, id int64, status string) (*TypeView, error) {
	if status != model.StatusEnabled && status != model.StatusDisabled {
		return nil, repository.ErrValidation("status must be enabled or disabled")
	}
	repo := &repository.TicketTypes{Q: s.DB}
	if err := repo.SetStatus(ctx, id, status); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, err
		}
		return nil, err
	}
	t, err := repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toTypeView(t), nil
}
