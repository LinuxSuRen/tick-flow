// Package seed embeds the builtin template packs (issue #19) and imports
// them as the initial ticket types of a new tenant (issue #12).
package seed

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"

	"github.com/LinuxSuRen/tick-flow/internal/model"
	"github.com/LinuxSuRen/tick-flow/internal/repository"
	"github.com/LinuxSuRen/tick-flow/internal/templating"
)

// FS holds every seed pack. Files are the test fixtures of the templating
// validators (issue #19: fixtures double as test cases).
//
//go:embed *.yaml
var FS embed.FS

// Importer implements service.Seeder over the embedded packs.
type Importer struct{}

// Import copies every seed pack as builtin types of the tenant. Conflicts
// are skipped (idempotent re-runs, e.g. a retried setup transaction).
func (Importer) Import(ctx context.Context, q repository.Querier, tenantID int64) error {
	names, err := fs.Glob(FS, "*.yaml")
	if err != nil {
		return fmt.Errorf("glob seed packs: %w", err)
	}
	sort.Strings(names)
	types := &repository.TicketTypes{Q: q}
	for _, name := range names {
		raw, err := FS.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read seed pack %s: %w", name, err)
		}
		pack, err := templating.ParsePackYAML(raw)
		if err != nil {
			return fmt.Errorf("parse seed pack %s: %w", name, err)
		}
		if ve := pack.Validate(); !ve.Empty() {
			return fmt.Errorf("seed pack %s invalid: %w", name, ve)
		}
		if err := types.CreateForTenant(ctx, tenantID, &model.TicketType{
			Slug: pack.Slug, Name: pack.Name, Icon: pack.Icon,
			FormSchema: pack.FormSchemaJSON(), Workflow: pack.WorkflowJSON(),
			SLAPolicy: pack.SLAPolicyJSON(), Assignment: pack.AssignmentJSON(),
			ViewConfig: pack.ViewConfigJSON(),
			IsBuiltin:  true, Status: model.StatusEnabled,
		}); err != nil {
			return fmt.Errorf("import seed pack %s: %w", name, err)
		}
		slog.Info("seed pack imported", "tenant_id", tenantID, "pack", pack.Slug)
	}
	return nil
}

// Packs parses and returns every embedded pack (tests iterate these).
func Packs() ([]*templating.Pack, error) {
	names, err := fs.Glob(FS, "*.yaml")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]*templating.Pack, 0, len(names))
	for _, name := range names {
		raw, err := FS.ReadFile(name)
		if err != nil {
			return nil, err
		}
		pack, err := templating.ParsePackYAML(raw)
		if err != nil {
			return nil, fmt.Errorf("parse seed pack %s: %w", name, err)
		}
		out = append(out, pack)
	}
	return out, nil
}
