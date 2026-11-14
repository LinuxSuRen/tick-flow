package service

import (
	"context"
	"database/sql"
	"errors"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/events"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
)

// MaxCommentBytes bounds comment bodies (issue #14).
const MaxCommentBytes = 16 << 10

// ErrCommentTooLarge reports an over-limit comment body.
var ErrCommentTooLarge = errors.New("comment body exceeds 16KB")

// ErrInternalNoteForbidden reports a requester attempting an internal note.
var ErrInternalNoteForbidden = errors.New("internal notes require agent or tenant_admin")

// TimelineService implements comments and timeline reads (issue #14).
type TimelineService struct {
	DB  *sql.DB
	Bus *events.Bus
}

// AddComment posts a reply or internal note on a ticket.
func (s *TimelineService) AddComment(ctx context.Context, ticketID int64, body string, isInternal bool) (*repository.EntryView, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, err
	}
	if len(body) > MaxCommentBytes {
		return nil, ErrCommentTooLarge
	}
	if body == "" {
		return nil, repository.ErrValidation("comment body is required")
	}
	ticket, err := (&repository.Tickets{Q: s.DB}).ByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ac.Role == model.RoleRequester && ticket.CreatorID != ac.UserID {
		return nil, ErrTicketNotFound
	}
	if isInternal && ac.Role == model.RoleRequester {
		return nil, ErrInternalNoteForbidden
	}
	kind := model.KindComment
	if isInternal {
		kind = model.KindInternalNote
	}

	var entryID int64
	err = repository.WithTx(ctx, s.DB, func(tx *sql.Tx) error {
		var err error
		entryID, err = (&repository.Timeline{Q: tx}).Insert(ctx, ticketID, kind, ac.UserID, body, nil)
		return err
	})
	if err != nil {
		return nil, err
	}

	s.Bus.Publish(ctx, events.New(events.TopicTicketCommented, ac.TenantID, map[string]any{
		"ticket_id":   ticket.ID,
		"number":      ticket.Number,
		"title":       ticket.Title,
		"comment_id":  entryID,
		"author_id":   ac.UserID,
		"is_internal": isInternal,
	}))

	author, _ := (&repository.Users{Q: s.DB}).ByID(ctx, ac.UserID)
	name := ""
	if author != nil {
		name = author.DisplayName
	}
	return &repository.EntryView{
		ID: entryID, Kind: kind, AuthorID: ac.UserID, Author: name, Body: body,
		CreatedAt: nowUTC(),
	}, nil
}

// List pages the timeline; requesters never see internal notes (issue #14).
func (s *TimelineService) List(ctx context.Context, ticketID int64, page, pageSize int) ([]repository.EntryView, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	ticket, err := (&repository.Tickets{Q: s.DB}).ByID(ctx, ticketID)
	if err != nil {
		return nil, 0, err
	}
	if ac.Role == model.RoleRequester && ticket.CreatorID != ac.UserID {
		return nil, 0, ErrTicketNotFound
	}
	includeInternal := ac.Role == model.RoleAgent || ac.Role == model.RoleTenantAdmin
	return (&repository.Timeline{Q: s.DB}).ListByTicket(ctx, ticketID, includeInternal, page, pageSize)
}
