-- TickFlow initial schema (issue #2). All business tables carry tenant_id,
-- created_at and updated_at where the design mandates them.
CREATE TABLE tenants (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    logo_url    TEXT,
    theme       JSONB NOT NULL DEFAULT '{}',
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name  TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE tenant_members (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  BIGINT NOT NULL REFERENCES tenants(id),
    user_id    BIGINT NOT NULL REFERENCES users(id),
    role       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, user_id)
);

CREATE TABLE ticket_types (
    id          BIGSERIAL PRIMARY KEY,
    tenant_id   BIGINT NOT NULL REFERENCES tenants(id),
    slug        TEXT NOT NULL,
    name        TEXT NOT NULL,
    icon        TEXT,
    form_schema JSONB NOT NULL DEFAULT '{}',
    workflow    JSONB NOT NULL DEFAULT '{}',
    sla_policy  JSONB NOT NULL DEFAULT '{}',
    assignment  JSONB NOT NULL DEFAULT '{}',
    view_config JSONB NOT NULL DEFAULT '{}',
    is_builtin  BOOLEAN NOT NULL DEFAULT false,
    status      TEXT NOT NULL DEFAULT 'enabled',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, slug)
);

CREATE TABLE tickets (
    id             BIGSERIAL PRIMARY KEY,
    tenant_id      BIGINT NOT NULL REFERENCES tenants(id),
    ticket_type_id BIGINT NOT NULL REFERENCES ticket_types(id),
    number         TEXT NOT NULL,
    title          TEXT NOT NULL,
    content        TEXT NOT NULL DEFAULT '',
    priority       TEXT NOT NULL DEFAULT 'normal',
    state          TEXT NOT NULL,
    custom_fields  JSONB NOT NULL DEFAULT '{}',
    creator_id     BIGINT NOT NULL REFERENCES users(id),
    assignee_id    BIGINT REFERENCES users(id),
    due_at         TIMESTAMPTZ,
    sla_deadlines  JSONB NOT NULL DEFAULT '{}',
    closed_at      TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, number)
);

CREATE INDEX idx_tickets_tenant_state    ON tickets (tenant_id, state);
CREATE INDEX idx_tickets_tenant_assignee ON tickets (tenant_id, assignee_id);
CREATE INDEX idx_tickets_tenant_type     ON tickets (tenant_id, ticket_type_id);

CREATE TABLE ticket_timeline (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    ticket_id  BIGINT NOT NULL REFERENCES tickets(id),
    kind       TEXT NOT NULL,
    author_id  BIGINT NOT NULL REFERENCES users(id),
    body       TEXT NOT NULL DEFAULT '',
    meta       JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_timeline_ticket ON ticket_timeline (ticket_id, created_at);

CREATE TABLE attachments (
    id           BIGSERIAL PRIMARY KEY,
    tenant_id    BIGINT NOT NULL,
    ticket_id    BIGINT NOT NULL REFERENCES tickets(id),
    timeline_id  BIGINT REFERENCES ticket_timeline(id),
    filename     TEXT NOT NULL,
    size_bytes   BIGINT NOT NULL,
    content_type TEXT NOT NULL,
    storage_path TEXT NOT NULL,
    sha256       TEXT NOT NULL,
    uploader_id  BIGINT NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE notifications (
    id         BIGSERIAL PRIMARY KEY,
    tenant_id  BIGINT NOT NULL,
    user_id    BIGINT NOT NULL REFERENCES users(id),
    ticket_id  BIGINT REFERENCES tickets(id),
    kind       TEXT NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    read_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_unread ON notifications (user_id) WHERE read_at IS NULL;

-- Per-tenant ticket number sequence, consumed with SELECT ... FOR UPDATE to
-- guarantee uniqueness under concurrent creation (issue #2 constraint 5).
CREATE TABLE ticket_sequences (
    tenant_id  BIGINT PRIMARY KEY REFERENCES tenants(id),
    next_value BIGINT NOT NULL DEFAULT 1
);
