// Package store provides data access for the Prompt & Cost Platform.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/fxwio/prompt-cost/internal/model"
	"github.com/fxwio/prompt-cost/pkg/pgstore"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errDBUnavailable is returned when the connection pool has not been initialized.
var errDBUnavailable = errors.New("pgstore: database not available")

// db returns the pool or errDBUnavailable. Every store function calls this so
// that a missing pgstore.Init() becomes a clean error instead of a nil-deref.
func db() (*pgxpool.Pool, error) {
	p := pgstore.Pool()
	if p == nil {
		return nil, errDBUnavailable
	}
	return p, nil
}

// ── Templates ─────────────────────────────────────────────────────────────────

func CreateTemplate(ctx context.Context, t *model.Template) (*model.Template, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	const q = `
		INSERT INTO templates (name, description, tags)
		VALUES ($1, $2, $3)
		RETURNING id, name, description, tags, active_version, version_count, created_at, updated_at`
	var out model.Template
	if err := p.QueryRow(ctx, q, t.Name, t.Description, t.Tags).Scan(
		&out.ID, &out.Name, &out.Description, &out.Tags,
		&out.ActiveVersion, &out.VersionCount, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("store.CreateTemplate: %w", err)
	}
	return &out, nil
}

func GetTemplate(ctx context.Context, id string) (*model.Template, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	const q = `SELECT id, name, description, tags, active_version, version_count, created_at, updated_at
               FROM templates WHERE id = $1`
	var t model.Template
	if err := p.QueryRow(ctx, q, id).Scan(
		&t.ID, &t.Name, &t.Description, &t.Tags,
		&t.ActiveVersion, &t.VersionCount, &t.CreatedAt, &t.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("store.GetTemplate %q: %w", id, err)
	}
	return &t, nil
}

func ListTemplates(ctx context.Context) ([]model.Template, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT id, name, description, tags, active_version, version_count, created_at, updated_at
		FROM templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store.ListTemplates: %w", err)
	}
	defer rows.Close()
	var out []model.Template
	for rows.Next() {
		var t model.Template
		if err := rows.Scan(&t.ID, &t.Name, &t.Description, &t.Tags,
			&t.ActiveVersion, &t.VersionCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func DeleteTemplate(ctx context.Context, id string) error {
	p, err := db()
	if err != nil {
		return err
	}
	_, err = p.Exec(ctx, `DELETE FROM templates WHERE id = $1`, id)
	return err
}

// ── Template Versions ─────────────────────────────────────────────────────────

// CreateVersion saves a new version and increments version_count.
// Returns the newly created version.
func CreateVersion(ctx context.Context, templateID, content string, variables []string) (*model.TemplateVersion, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	tx, err := p.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Increment version_count and get the new version number atomically
	var nextVersion int
	if err := tx.QueryRow(ctx, `
		UPDATE templates
		SET version_count = version_count + 1, updated_at = NOW()
		WHERE id = $1
		RETURNING version_count`, templateID,
	).Scan(&nextVersion); err != nil {
		return nil, fmt.Errorf("increment version_count: %w", err)
	}

	var v model.TemplateVersion
	if err := tx.QueryRow(ctx, `
		INSERT INTO template_versions (template_id, version, content, variables)
		VALUES ($1, $2, $3, $4)
		RETURNING id, template_id, version, content, variables, created_at`,
		templateID, nextVersion, content, variables,
	).Scan(&v.ID, &v.TemplateID, &v.Version, &v.Content, &v.Variables, &v.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Auto-activate first version
	if nextVersion == 1 {
		if _, err := tx.Exec(ctx,
			`UPDATE templates SET active_version = 1 WHERE id = $1`, templateID,
		); err != nil {
			return nil, err
		}
	}

	return &v, tx.Commit(ctx)
}

func GetVersion(ctx context.Context, templateID string, version int) (*model.TemplateVersion, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	const q = `SELECT id, template_id, version, content, variables, created_at
               FROM template_versions WHERE template_id = $1 AND version = $2`
	var v model.TemplateVersion
	if err := p.QueryRow(ctx, q, templateID, version).Scan(
		&v.ID, &v.TemplateID, &v.Version, &v.Content, &v.Variables, &v.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("store.GetVersion(%q, %d): %w", templateID, version, err)
	}
	return &v, nil
}

func ListVersions(ctx context.Context, templateID string) ([]model.TemplateVersion, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	rows, err := p.Query(ctx, `
		SELECT id, template_id, version, content, variables, created_at
		FROM template_versions WHERE template_id = $1 ORDER BY version`, templateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TemplateVersion
	for rows.Next() {
		var v model.TemplateVersion
		if err := rows.Scan(&v.ID, &v.TemplateID, &v.Version,
			&v.Content, &v.Variables, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// ActivateVersion sets the active version for a template.
func ActivateVersion(ctx context.Context, templateID string, version int) error {
	p, err := db()
	if err != nil {
		return err
	}
	result, err := p.Exec(ctx, `
		UPDATE templates SET active_version = $1, updated_at = NOW()
		WHERE id = $2 AND EXISTS (
			SELECT 1 FROM template_versions
			WHERE template_id = $2 AND version = $1
		)`, version, templateID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("version %d not found for template %q", version, templateID)
	}
	return nil
}

// ── Usage Events ──────────────────────────────────────────────────────────────

func RecordUsage(ctx context.Context, e *model.UsageEvent) (*model.UsageEvent, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	meta, _ := json.Marshal(e.Metadata)
	const q = `
		INSERT INTO usage_events (tenant, app, model, prompt_tokens, completion_tokens, cost_usd, metadata)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, tenant, app, model, prompt_tokens, completion_tokens,
		          cost_usd::float8, metadata, created_at`
	var out model.UsageEvent
	var metaRaw []byte
	if err := p.QueryRow(ctx, q,
		e.Tenant, e.App, e.Model, e.PromptTokens, e.CompletionTokens, e.CostUSD, string(meta),
	).Scan(
		&out.ID, &out.Tenant, &out.App, &out.Model,
		&out.PromptTokens, &out.CompletionTokens, &out.CostUSD, &metaRaw, &out.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("store.RecordUsage: %w", err)
	}
	_ = json.Unmarshal(metaRaw, &out.Metadata)
	return &out, nil
}

func ListUsageEvents(ctx context.Context, tenant, app, mdl string, from, to time.Time, limit, offset int) ([]model.UsageEvent, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	query := `SELECT id, tenant, app, model, prompt_tokens, completion_tokens,
	                 cost_usd::float8, metadata, created_at
	          FROM usage_events WHERE 1=1`
	args := []any{}
	n := 1

	if tenant != "" {
		query += fmt.Sprintf(" AND tenant = $%d", n)
		args = append(args, tenant)
		n++
	}
	if app != "" {
		query += fmt.Sprintf(" AND app = $%d", n)
		args = append(args, app)
		n++
	}
	if mdl != "" {
		query += fmt.Sprintf(" AND model = $%d", n)
		args = append(args, mdl)
		n++
	}
	if !from.IsZero() {
		query += fmt.Sprintf(" AND created_at >= $%d", n)
		args = append(args, from)
		n++
	}
	if !to.IsZero() {
		query += fmt.Sprintf(" AND created_at <= $%d", n)
		args = append(args, to)
		n++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", n, n+1)
	args = append(args, limit, offset)

	rows, err := p.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store.ListUsageEvents: %w", err)
	}
	defer rows.Close()
	var out []model.UsageEvent
	for rows.Next() {
		var e model.UsageEvent
		var metaRaw []byte
		if err := rows.Scan(&e.ID, &e.Tenant, &e.App, &e.Model,
			&e.PromptTokens, &e.CompletionTokens, &e.CostUSD, &metaRaw, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(metaRaw, &e.Metadata)
		out = append(out, e)
	}
	return out, rows.Err()
}

// BuildCostReport aggregates usage events into a cost report.
func BuildCostReport(ctx context.Context, groupBy model.GroupBy, from, to time.Time) (*model.CostReport, error) {
	p, err := db()
	if err != nil {
		return nil, err
	}
	var groupCol string
	switch groupBy {
	case model.GroupByTenant:
		groupCol = "tenant"
	case model.GroupByApp:
		groupCol = "app"
	case model.GroupByModel:
		groupCol = "model"
	default:
		groupCol = "model"
	}

	query := fmt.Sprintf(`
		SELECT %s AS key,
		       COUNT(*)                  AS event_count,
		       COALESCE(SUM(prompt_tokens), 0)     AS prompt_tokens,
		       COALESCE(SUM(completion_tokens), 0) AS completion_tokens,
		       COALESCE(SUM(cost_usd), 0)::float8  AS cost_usd
		FROM usage_events
		WHERE created_at BETWEEN $1 AND $2
		GROUP BY %s
		ORDER BY cost_usd DESC`, groupCol, groupCol)

	rows, err := p.Query(ctx, query, from, to)
	if err != nil {
		return nil, fmt.Errorf("store.BuildCostReport: %w", err)
	}
	defer rows.Close()

	report := &model.CostReport{
		From:    from,
		To:      to,
		GroupBy: groupBy,
	}
	for rows.Next() {
		var g model.CostGroup
		if err := rows.Scan(&g.Key, &g.EventCount, &g.PromptTokens,
			&g.CompletionTokens, &g.CostUSD); err != nil {
			return nil, err
		}
		g.TotalTokens = g.PromptTokens + g.CompletionTokens
		if g.EventCount > 0 {
			g.AvgCostPerEvent = g.CostUSD / float64(g.EventCount)
		}
		report.Groups = append(report.Groups, g)
		report.TotalEvents += g.EventCount
		report.TotalPromptTokens += g.PromptTokens
		report.TotalCompletionTokens += g.CompletionTokens
		report.TotalCostUSD += g.CostUSD
	}
	return report, rows.Err()
}
