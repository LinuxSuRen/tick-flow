// Package app assembles the full application: config, database, event bus,
// services, handlers and the router. main.go and the integration tests
// share this wiring so production and test behavior cannot drift apart.
package app

import (
	"database/sql"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/LinuxSuRen/tick-flow/internal/config"
	"github.com/LinuxSuRen/tick-flow/internal/events"
	"github.com/LinuxSuRen/tick-flow/internal/handler"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/router"
	"github.com/LinuxSuRen/tick-flow/internal/service"
	"github.com/LinuxSuRen/tick-flow/internal/storage"
	"github.com/LinuxSuRen/tick-flow/seed"
)

// App owns every long-lived collaborator of the server.
type App struct {
	Config config.Config
	DB     *sql.DB
	Bus    *events.Bus
	Engine *gin.Engine

	Store *storage.Local
}

// New wires the application from the configuration (migrations included).
func New(cfg config.Config, migrationsFS fs.FS) (*App, error) {
	db, err := repository.Open(cfg.DBDSN)
	if err != nil {
		return nil, err
	}
	if err := repository.Migrate(db, migrationsFS); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	a, err := NewWithDB(cfg, db)
	if err != nil {
		db.Close()
		return nil, err
	}
	return a, nil
}

// NewWithDB wires the application over an existing pool (tests reuse the
// container across cases by resetting data between them).
func NewWithDB(cfg config.Config, db *sql.DB) (*App, error) {
	bus := events.NewBus()
	mailer := service.NewMailer(cfg.SMTP)
	store, err := storage.NewLocal(cfg.UploadDir)
	if err != nil {
		return nil, err
	}

	engine := router.New(router.Deps{
		DB:        db,
		JWTSecret: cfg.JWTSecret,
		Auth: &handler.Auth{Svc: &service.AuthService{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Seeder:    seed.Importer{},
			Mailer:    mailer,
		}},
		Tenant: &handler.Tenant{Svc: &service.TenantService{DB: db}},
		Member: &handler.Member{Svc: &service.MemberService{
			DB:        db,
			JWTSecret: cfg.JWTSecret,
			Mailer:    mailer,
		}},
		TicketType:     &handler.TicketType{Svc: &service.TicketTypeService{DB: db}},
		Ticket:         &handler.Ticket{Svc: &service.TicketService{DB: db, Bus: bus}},
		Timeline:       &handler.Timeline{Svc: &service.TimelineService{DB: db, Bus: bus}},
		Attachment:     &handler.Attachment{Svc: service.NewAttachmentService(db, bus, store, cfg.UploadMax, cfg.UploadExts)},
		Notification:   &handler.Notification{Svc: &service.NotificationService{DB: db}},
	})

	bus.Register(service.InAppSubscriber{DB: db})
	bus.Register(service.MailSubscriber{DB: db, Mailer: mailer})

	return &App{Config: cfg, DB: db, Bus: bus, Engine: engine, Store: store}, nil
}

// Close releases the resources.
func (a *App) Close() {
	if a.DB != nil {
		_ = a.DB.Close()
	}
}

// Reset truncates every business table between test cases. Tests run
// sequentially against one container for speed.
func (a *App) Reset() error {
	stmts := []string{
		`TRUNCATE notifications, attachments, ticket_timeline, tickets, ticket_sequences, ticket_types, tenant_members, users, tenants RESTART IDENTITY CASCADE`,
	}
	for _, s := range stmts {
		if _, err := a.DB.Exec(s); err != nil {
			return err
		}
	}
	slog.Info("test data reset")
	return nil
}
