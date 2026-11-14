package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/LinuxSuRen/tick-flow/internal/appctx"
	"github.com/LinuxSuRen/tick-flow/internal/events"
	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/templating"
)

// NotificationService exposes the notification reads for the handler; the
// subscribers below plug into the event bus (issue #16).
type NotificationService struct {
	DB *sql.DB
}

// List pages the notifications of the authenticated user.
func (s *NotificationService) List(ctx context.Context, unreadOnly bool, page, pageSize int) ([]repository.NotificationView, int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return nil, 0, err
	}
	return (&repository.Notifications{Q: s.DB}).ListByUser(ctx, ac.UserID, unreadOnly, page, pageSize)
}

// MarkRead marks ids (or everything) read for the authenticated user.
func (s *NotificationService) MarkRead(ctx context.Context, ids []int64) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	return (&repository.Notifications{Q: s.DB}).MarkRead(ctx, ac.UserID, ids)
}

// CountUnread returns the unread count of the authenticated user.
func (s *NotificationService) CountUnread(ctx context.Context) (int64, error) {
	ac, err := appctx.MustFrom(ctx)
	if err != nil {
		return 0, err
	}
	return (&repository.Notifications{Q: s.DB}).CountUnread(ctx, ac.UserID)
}

// recipientContext re-scopes the event into a tenant context so repository
// queries keep their mandatory tenant filter (issue #6 discipline holds in
// subscribers too).
func recipientContext(ctx context.Context, ev *events.Event) (context.Context, *appctx.AuthContext) {
	ac := &appctx.AuthContext{TenantID: ev.TenantID}
	return appctx.With(ctx, ac), ac
}

// ticketBrief loads what recipients and copy assembly need.
type ticketBrief struct {
	ID       int64
	TypeID   int64
	Number   string
	Title    string
	Creator  int64
	Assignee *int64
	TypeSlug string
}

func loadTicketBrief(ctx context.Context, q repository.Querier, tenantID, ticketID int64) (*ticketBrief, error) {
	t, err := (&repository.Tickets{Q: q}).ByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	tb := &ticketBrief{
		ID: t.ID, TypeID: t.TicketTypeID, Number: t.Number, Title: t.Title,
		Creator: t.CreatorID, Assignee: t.AssigneeID,
	}
	if tt, err := (&repository.TicketTypes{Q: q}).ByID(ctx, t.TicketTypeID); err == nil {
		tb.TypeSlug = tt.Slug
	}
	return tb, nil
}

// eventRecipients applies the P0 recipient table (issue #7): every topic
// excludes the operator/author so nobody gets notified about their own
// action (issue #16 acceptance).
func eventRecipients(ctx context.Context, q repository.Querier, ev *events.Event) ([]int64, *ticketBrief, string, error) {
	sctx, _ := recipientContext(ctx, ev)
	// The acting user (operator/author/uploader) never gets notified about
	// their own action (issue #16 acceptance).
	exclude := int64(0)
	for _, key := range []string{"operator_id", "author_id", "uploader_id"} {
		if v, ok := ev.Payload[key].(int64); ok && v != 0 {
			exclude = v
			break
		}
	}
	ticketID, _ := ev.Payload["ticket_id"].(int64)
	tb, err := loadTicketBrief(sctx, q, ev.TenantID, ticketID)
	if err != nil {
		return nil, nil, "", err
	}

	var ids []int64
	members := &repository.Members{Q: q}
	switch ev.Topic {
	case events.TopicTicketCreated:
		ids, err = members.AgentsAndAdmins(sctx, ev.TenantID)
	case events.TopicTicketAssigned:
		if v, _ := ev.Payload["assignee_id"].(int64); v != 0 {
			ids = []int64{v}
		}
	case events.TopicTicketStateChanged, events.TopicTicketCommented, events.TopicTicketAttachmentAdded:
		ids = append(ids, tb.Creator)
		if tb.Assignee != nil {
			ids = append(ids, *tb.Assignee)
		}
	default:
		return nil, tb, "", nil
	}
	if err != nil {
		return nil, nil, "", err
	}

	// dedupe + exclude the actor
	seen := map[int64]struct{}{}
	out := ids[:0]
	for _, id := range ids {
		if id == 0 || id == exclude {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, tb, stateNameOf(ctx, q, ev, tb), nil
}

// stateNameOf resolves the display name of the new state when the event
// carries one (copy data, not UI copy).
func stateNameOf(ctx context.Context, q repository.Querier, ev *events.Event, tb *ticketBrief) string {
	to, _ := ev.Payload["to"].(string)
	if to == "" || tb == nil || tb.TypeID == 0 {
		return ""
	}
	sctx, _ := recipientContext(ctx, ev)
	tt, err := (&repository.TicketTypes{Q: q}).ByID(sctx, tb.TypeID)
	if err != nil {
		return to
	}
	w, err := templating.ParseWorkflow(tt.Workflow)
	if err != nil {
		return to
	}
	return w.StateName(to)
}

// notificationCopy assembles the data content per topic (server-side data,
// the frontend still owns presentation).
func notificationCopy(ev *events.Event, tb *ticketBrief, stateName string) (kind, title, body string) {
	label := ""
	if tb != nil {
		label = fmt.Sprintf("工单 #%s %s", tb.Number, tb.Title)
	}
	switch ev.Topic {
	case events.TopicTicketCreated:
		return ev.Topic, "新工单：" + label, "请及时受理。"
	case events.TopicTicketAssigned:
		return ev.Topic, "工单已分派给你：" + label, "请在工作台查看详情。"
	case events.TopicTicketStateChanged:
		name := stateName
		if name == "" {
			name, _ = ev.Payload["to"].(string)
		}
		return ev.Topic, label + " 状态更新为「" + name + "」", ""
	case events.TopicTicketCommented:
		if internal, _ := ev.Payload["is_internal"].(bool); internal {
			return ev.Topic, "工单有新的内部备注：" + label, ""
		}
		return ev.Topic, "工单有新回复：" + label, ""
	case events.TopicTicketAttachmentAdded:
		filename, _ := ev.Payload["filename"].(string)
		return ev.Topic, "工单有新附件：" + label, filename
	default:
		return ev.Topic, label, ""
	}
}

// InAppSubscriber writes the notifications table.
type InAppSubscriber struct {
	DB *sql.DB
}

// Name implements events.Subscriber.
func (InAppSubscriber) Name() string { return "in_app" }

// Handle implements events.Subscriber.
func (s InAppSubscriber) Handle(ctx context.Context, ev *events.Event) error {
	ids, tb, stateName, err := eventRecipients(ctx, s.DB, ev)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	kind, title, body := notificationCopy(ev, tb, stateName)
	ticketID, _ := ev.Payload["ticket_id"].(int64)
	var ticketPtr *int64
	if ticketID != 0 {
		ticketPtr = &ticketID
	}
	return (&repository.Notifications{Q: s.DB}).InsertBatch(ctx, ev.TenantID, ids, kind, title, body, ticketPtr)
}

// MailSubscriber sends plain-text mail per the same recipient rules; mail
// failures are logged and never propagate (issue #16).
type MailSubscriber struct {
	DB     *sql.DB
	Mailer *Mailer
}

// Name implements events.Subscriber.
func (s MailSubscriber) Name() string { return "mail" }

// Handle implements events.Subscriber.
func (s MailSubscriber) Handle(ctx context.Context, ev *events.Event) error {
	if !s.Mailer.Enabled() {
		return nil
	}
	ids, tb, stateName, err := eventRecipients(ctx, s.DB, ev)
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	_, title, body := notificationCopy(ev, tb, stateName)
	users := &repository.Users{Q: s.DB}
	for _, id := range ids {
		u, err := users.ByID(ctx, id)
		if err != nil {
			slog.Error("load mail recipient failed", "user_id", id, "error", err)
			continue
		}
		if u.Status != model.UserStatusActive {
			continue
		}
		if err := s.Mailer.Send(u.Email, title, body); err != nil {
			slog.Error("send notification mail failed", "to", u.Email, "error", err)
		}
	}
	return nil
}
