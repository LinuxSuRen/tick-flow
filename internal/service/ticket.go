package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/events"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/templating"
)

// Ticket errors mapped by the handler layer.
var (
	ErrTicketNotFound  = repository.ErrNotFound
	ErrIllegalAction   = errors.New("action is not available in the current state")
	ErrRoleDenied      = errors.New("role is not allowed to perform this action")
	ErrAssigneeInvalid = errors.New("assignee must be an agent or tenant_admin of this tenant")
)

// TicketService implements creation, listing, detail, transitions and
// assignment (issue #13).
type TicketService struct {
	DB  *sql.DB
	Bus *events.Bus
}

// TicketView is the API projection of a ticket.
type TicketView struct {
	ID           int64           `json:"id"`
	Number       string          `json:"number"`
	TicketType   TypeSummary     `json:"ticket_type"`
	Title        string          `json:"title"`
	Content      string          `json:"content"`
	Priority     string          `json:"priority"`
	State        string          `json:"state"`
	StateName    string          `json:"state_name"`
	CustomFields json.RawMessage `json:"custom_fields"`
	CreatorID    int64           `json:"creator_id"`
	AssigneeID   *int64          `json:"assignee_id"`
	DueAt        *string         `json:"due_at"`
	SLADeadlines json.RawMessage `json:"sla_deadlines"`
	ClosedAt     *string         `json:"closed_at"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`
}

// TypeSummary carries what the frontend needs to render one ticket.
type TypeSummary struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
	Icon string `json:"icon"`
}

// TransitionView describes one available action from the current state.
type TransitionView struct {
	Action string `json:"action"`
	To     string `json:"to"`
	ToName string `json:"to_name"`
}

// Detail extends TicketView with the transitions available to the caller
// and the ticket attachments.
type Detail struct {
	TicketView
	Transitions []TransitionView   `json:"transitions"`
	Attachments []AttachmentBrief  `json:"attachments"`
}

// AttachmentBrief is the attachment projection inside a ticket detail.
type AttachmentBrief struct {
	ID          int64  `json:"id"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
	CreatedAt   string `json:"created_at"`
}

// CreateRequest is the POST /api/tickets payload.
type CreateRequest struct {
	TicketTypeID int64          `json:"ticket_type_id"`
	Title        string         `json:"title"`
	Content      string         `json:"content"`
	Priority     string         `json:"priority"`
	CustomFields map[string]any `json:"custom_fields"`
}

// Create validates custom fields against the type form schema and creates
// the ticket with the workflow initial state (issue #13).
func (s *TicketService) Create(ctx context.Context, req CreateRequest) (*Detail, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	if req.Title == "" || len(req.Title) > 200 {
		return nil, repository.ErrValidation("title is required and must be at most 200 characters")
	}
	priority := req.Priority
	if priority == "" {
		priority = model.PriorityNormal
	}
	if !validPriorities[priority] {
		return nil, repository.ErrValidation("priority must be low, normal, high or urgent")
	}
	if req.CustomFields == nil {
		req.CustomFields = map[string]any{}
	}

	tt, err := (&repository.TicketTypes{Q: s.DB}).ByID(ctx, req.TicketTypeID)
	if err != nil {
		return nil, err
	}
	if tt.Status != model.StatusEnabled {
		return nil, repository.ErrValidation("ticket type is disabled")
	}
	schema, err := templating.ParseFormSchema(tt.FormSchema)
	if err != nil {
		return nil, repository.ErrValidation("stored form schema is invalid: " + err.Error())
	}
	if ve := schema.ValidateData(req.CustomFields); !ve.Empty() {
		return nil, ve
	}
	workflow, err := templating.ParseWorkflow(tt.Workflow)
	if err != nil {
		return nil, repository.ErrValidation("stored workflow is invalid: " + err.Error())
	}
	initial, err := workflow.InitialState()
	if err != nil {
		return nil, repository.ErrValidation(err.Error())
	}

	fieldsJSON, err := json.Marshal(req.CustomFields)
	if err != nil {
		return nil, err
	}
	deadlines, err := slaDeadlines(tt.SLAPolicy)
	if err != nil {
		return nil, err
	}

	var ticket *model.Ticket
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		ticket, err = (&repository.Tickets{Q: tx}).Create(ctx, &model.Ticket{
			TicketTypeID: tt.ID,
			Title:        req.Title,
			Content:      req.Content,
			Priority:     priority,
			State:        initial,
			CustomFields: json.RawMessage(fieldsJSON),
			CreatorID:    ac.UserID,
			SLADeadlines: deadlines,
		})
		if err != nil {
			return err
		}
		// The timeline is the single source of truth: creation is a system
		// event too (issue #2).
		_, err = (&repository.Timeline{Q: tx}).Insert(ctx, ticket.ID, model.KindSystem, ac.UserID, "创建了工单", nil)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.Bus.Publish(ctx, events.New(events.TopicTicketCreated, ac.TenantID, map[string]any{
		"ticket_id":  ticket.ID,
		"number":     ticket.Number,
		"type":       tt.Slug,
		"type_id":    tt.ID,
		"title":      ticket.Title,
		"creator_id": ticket.CreatorID,
	}))
	return s.detail(ctx, ticket, tt, workflow)
}

var validPriorities = map[string]bool{
	model.PriorityLow: true, model.PriorityNormal: true,
	model.PriorityHigh: true, model.PriorityUrgent: true,
}

func slaDeadlines(raw []byte) (json.RawMessage, error) {
	if len(raw) == 0 {
		return json.RawMessage("{}"), nil
	}
	policy, err := templating.ParseSLAPolicy(raw)
	if err != nil {
		return nil, repository.ErrValidation("stored sla policy is invalid: " + err.Error())
	}
	now := time.Now().UTC()
	out := map[string]string{}
	if policy.FirstResponseMinutes != nil {
		out["first_response_at"] = now.Add(time.Duration(*policy.FirstResponseMinutes) * time.Minute).Format(time.RFC3339)
	}
	if policy.ResolveMinutes != nil {
		out["resolve_at"] = now.Add(time.Duration(*policy.ResolveMinutes) * time.Minute).Format(time.RFC3339)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// List pages tickets by filter; requesters are narrowed to their own
// tickets regardless of the filter (issue #13, #6).
func (s *TicketService) List(ctx context.Context, filter repository.TicketFilter, page, pageSize int) ([]*TicketView, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	if ac.Role == model.RoleRequester {
		filter.RequesterOnly = true
	}
	items, total, err := (&repository.Tickets{Q: s.DB}).List(ctx, filter, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	types := s.typeMap(ctx)
	out := make([]*TicketView, 0, len(items))
	for _, t := range items {
		out = append(out, s.toView(t, types))
	}
	return out, total, nil
}

// Get loads a ticket detail with caller-scoped transitions.
func (s *TicketService) Get(ctx context.Context, id int64) (*Detail, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	ticket, tt, workflow, err := s.loadTicket(ctx, id)
	if err != nil {
		return nil, err
	}
	if ac.Role == model.RoleRequester && ticket.CreatorID != ac.UserID {
		return nil, ErrTicketNotFound
	}
	return s.detail(ctx, ticket, tt, workflow)
}

// Transition applies an action: workflow lookup, role intersection, state
// update, timeline entry and event (issue #13).
func (s *TicketService) Transition(ctx context.Context, id int64, action, comment string) (*Detail, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	ticket, tt, workflow, err := s.loadTicket(ctx, id)
	if err != nil {
		return nil, err
	}
	if ac.Role == model.RoleRequester && ticket.CreatorID != ac.UserID {
		return nil, ErrTicketNotFound
	}
	tr, ok := workflow.TransitionFor(ticket.State, action)
	if !ok {
		return nil, ErrIllegalAction
	}
	if !tr.RolesAllow(ac.Role, ticket.CreatorID == ac.UserID) {
		return nil, ErrRoleDenied
	}
	isFinal := workflow.StateTypeOf(tr.To) == templating.StateTypeFinal

	var updated *model.Ticket
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		var err error
		updated, err = (&repository.Tickets{Q: tx}).UpdateState(ctx, id, tr.To, isFinal)
		if err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]string{
			"from":   ticket.State,
			"to":     tr.To,
			"action": action,
		})
		_, err = (&repository.Timeline{Q: tx}).Insert(ctx, id, model.KindStateChange, ac.UserID, comment, meta)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.Bus.Publish(ctx, events.New(events.TopicTicketStateChanged, ac.TenantID, map[string]any{
		"ticket_id":   updated.ID,
		"number":      updated.Number,
		"title":       updated.Title,
		"type":        tt.Slug,
		"from":        ticket.State,
		"to":          tr.To,
		"action":      action,
		"operator_id": ac.UserID,
	}))
	return s.detail(ctx, updated, tt, workflow)
}

// Assign sets the assignee; agent or tenant_admin only (enforced by the
// route middleware, membership re-checked here).
func (s *TicketService) Assign(ctx context.Context, id, assigneeID int64) (*Detail, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	_, tt, workflow, err := s.loadTicket(ctx, id)
	if err != nil {
		return nil, err
	}
	role, err := (&repository.Members{Q: s.DB}).Membership(ctx, ac.TenantID, assigneeID)
	if err != nil || (role != model.RoleAgent && role != model.RoleTenantAdmin) {
		return nil, ErrAssigneeInvalid
	}

	var updated *model.Ticket
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		var err error
		updated, err = (&repository.Tickets{Q: tx}).Assign(ctx, id, assigneeID)
		if err != nil {
			return err
		}
		meta, _ := json.Marshal(map[string]any{"assignee_id": assigneeID})
		_, err = (&repository.Timeline{Q: tx}).Insert(ctx, id, model.KindAssignment, ac.UserID, "", meta)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.Bus.Publish(ctx, events.New(events.TopicTicketAssigned, ac.TenantID, map[string]any{
		"ticket_id":   updated.ID,
		"number":      updated.Number,
		"title":       updated.Title,
		"type":        tt.Slug,
		"assignee_id": assigneeID,
		"operator_id": ac.UserID,
	}))
	return s.detail(ctx, updated, tt, workflow)
}

func (s *TicketService) loadTicket(ctx context.Context, id int64) (*model.Ticket, *model.TicketType, *templating.Workflow, error) {
	ticket, err := (&repository.Tickets{Q: s.DB}).ByID(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	tt, err := (&repository.TicketTypes{Q: s.DB}).ByID(ctx, ticket.TicketTypeID)
	if err != nil {
		return nil, nil, nil, err
	}
	workflow, err := templating.ParseWorkflow(tt.Workflow)
	if err != nil {
		return nil, nil, nil, repository.ErrValidation("stored workflow is invalid: " + err.Error())
	}
	return ticket, tt, workflow, nil
}

// detail assembles the full response including caller-visible transitions.
func (s *TicketService) detail(ctx context.Context, t *model.Ticket, tt *model.TicketType, workflow *templating.Workflow) (*Detail, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	view := s.toViewWith(t, tt, workflow)
	d := &Detail{TicketView: *view, Transitions: []TransitionView{}, Attachments: []AttachmentBrief{}}
	for _, tr := range workflow.AvailableTransitions(t.State) {
		if !tr.RolesAllow(ac.Role, t.CreatorID == ac.UserID) {
			continue
		}
		d.Transitions = append(d.Transitions, TransitionView{
			Action: tr.Action, To: tr.To, ToName: workflow.StateName(tr.To),
		})
	}
	atts, err := (&repository.Attachments{Q: s.DB}).ListByTicket(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	for _, a := range atts {
		d.Attachments = append(d.Attachments, AttachmentBrief{
			ID: a.ID, Filename: a.Filename, SizeBytes: a.SizeBytes,
			ContentType: a.ContentType,
			CreatedAt:  a.CreatedAt.UTC().Format(timeFormat),
		})
	}
	return d, nil
}

func (s *TicketService) typeMap(ctx context.Context) map[int64]*model.TicketType {
	items, err := (&repository.TicketTypes{Q: s.DB}).ListEnabled(ctx)
	if err != nil {
		return map[int64]*model.TicketType{}
	}
	out := make(map[int64]*model.TicketType, len(items))
	for _, t := range items {
		out[t.ID] = t
	}
	return out
}

func (s *TicketService) toView(t *model.Ticket, types map[int64]*model.TicketType) *TicketView {
	tt := types[t.TicketTypeID]
	return s.toViewWith(t, tt, nil)
}

func (s *TicketService) toViewWith(t *model.Ticket, tt *model.TicketType, workflow *templating.Workflow) *TicketView {
	v := &TicketView{
		ID: t.ID, Number: t.Number,
		TicketType:   TypeSummary{ID: t.TicketTypeID},
		Title:        t.Title, Content: t.Content,
		Priority:     t.Priority,
		State:        t.State,
		CustomFields: jsonOrDefault(t.CustomFields),
		CreatorID:    t.CreatorID,
		AssigneeID:   t.AssigneeID,
		SLADeadlines: jsonOrDefault(t.SLADeadlines),
		CreatedAt:    t.CreatedAt.UTC().Format(timeFormat),
		UpdatedAt:    t.UpdatedAt.UTC().Format(timeFormat),
	}
	if t.DueAt != nil {
		s := t.DueAt.UTC().Format(timeFormat)
		v.DueAt = &s
	}
	if t.ClosedAt != nil {
		s := t.ClosedAt.UTC().Format(timeFormat)
		v.ClosedAt = &s
	}
	if tt != nil {
		v.TicketType = TypeSummary{ID: tt.ID, Slug: tt.Slug, Name: tt.Name, Icon: tt.Icon}
	}
	if workflow != nil {
		v.StateName = workflow.StateName(t.State)
	} else if parsed, err := templating.ParseWorkflow(tt.Workflow); err == nil {
		v.StateName = parsed.StateName(t.State)
	}
	return v
}
