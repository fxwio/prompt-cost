//go:build integration

package integration_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/fxwio/prompt-cost/internal/model"
	"github.com/fxwio/prompt-cost/internal/store"
	"github.com/fxwio/prompt-cost/pkg/pgstore"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPostgres(t *testing.T) (dsn string, cleanup func()) {
	t.Helper()
	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "postgres:17-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "pc",
			"POSTGRES_PASSWORD": "pc",
			"POSTGRES_DB":       "prompt_cost",
		},
		WaitingFor: wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).WithStartupTimeout(60 * time.Second),
	}
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "5432")
	dsn = "postgres://pc:pc@" + host + ":" + port.Port() + "/prompt_cost?sslmode=disable"
	cleanup = func() { _ = container.Terminate(ctx) }
	return
}

func TestIntegration(t *testing.T) {
	dsn, cleanup := startPostgres(t)
	defer cleanup()

	ctx := context.Background()
	if err := pgstore.Init(ctx, dsn, 5, 1); err != nil {
		t.Fatalf("pgstore.Init: %v", err)
	}
	defer pgstore.Close()

	t.Run("TemplateLifecycle", func(t *testing.T) {
		// Create template
		tmpl, err := store.CreateTemplate(ctx, &model.Template{
			Name:        "test-template",
			Description: "integration test template",
			Tags:        []string{"test"},
		})
		if err != nil {
			t.Fatalf("CreateTemplate: %v", err)
		}
		if tmpl.ID == "" {
			t.Fatal("expected non-empty ID")
		}
		if tmpl.ActiveVersion != 0 {
			t.Errorf("new template should have active_version=0, got %d", tmpl.ActiveVersion)
		}

		// Get template
		got, err := store.GetTemplate(ctx, tmpl.ID)
		if err != nil {
			t.Fatalf("GetTemplate: %v", err)
		}
		if got.Name != tmpl.Name {
			t.Errorf("name: got %q want %q", got.Name, tmpl.Name)
		}

		// List templates
		list, err := store.ListTemplates(ctx)
		if err != nil {
			t.Fatalf("ListTemplates: %v", err)
		}
		if len(list) == 0 {
			t.Error("expected at least 1 template")
		}
	})

	t.Run("VersioningAndActivation", func(t *testing.T) {
		tmpl, _ := store.CreateTemplate(ctx, &model.Template{Name: "versioned-tmpl"})

		// Create version 1 — should auto-activate
		v1, err := store.CreateVersion(ctx, tmpl.ID,
			"Hello {{.name}}!", []string{"name"})
		if err != nil {
			t.Fatalf("CreateVersion v1: %v", err)
		}
		if v1.Version != 1 {
			t.Errorf("v1.Version: got %d want 1", v1.Version)
		}
		if len(v1.Variables) != 1 || v1.Variables[0] != "name" {
			t.Errorf("v1.Variables: got %v want [name]", v1.Variables)
		}

		// Template should now have active_version=1
		updated, _ := store.GetTemplate(ctx, tmpl.ID)
		if updated.ActiveVersion != 1 {
			t.Errorf("active_version after v1: got %d want 1", updated.ActiveVersion)
		}

		// Create version 2
		v2, err := store.CreateVersion(ctx, tmpl.ID,
			"Hi {{.name}}, you are {{.age}} years old!", []string{"name", "age"})
		if err != nil {
			t.Fatalf("CreateVersion v2: %v", err)
		}
		if v2.Version != 2 {
			t.Errorf("v2.Version: got %d want 2", v2.Version)
		}

		// Active version should still be 1 (only v1 auto-activates)
		updated2, _ := store.GetTemplate(ctx, tmpl.ID)
		if updated2.ActiveVersion != 1 {
			t.Errorf("active_version should still be 1, got %d", updated2.ActiveVersion)
		}

		// Activate version 2
		if err := store.ActivateVersion(ctx, tmpl.ID, 2); err != nil {
			t.Fatalf("ActivateVersion: %v", err)
		}
		updated3, _ := store.GetTemplate(ctx, tmpl.ID)
		if updated3.ActiveVersion != 2 {
			t.Errorf("active_version after activate: got %d want 2", updated3.ActiveVersion)
		}

		// Activate non-existent version should fail
		if err := store.ActivateVersion(ctx, tmpl.ID, 99); err == nil {
			t.Error("expected error activating non-existent version")
		}

		// List versions
		versions, err := store.ListVersions(ctx, tmpl.ID)
		if err != nil {
			t.Fatalf("ListVersions: %v", err)
		}
		if len(versions) != 2 {
			t.Errorf("version count: got %d want 2", len(versions))
		}

		// GetVersion
		got, err := store.GetVersion(ctx, tmpl.ID, 1)
		if err != nil {
			t.Fatalf("GetVersion: %v", err)
		}
		if got.Content != "Hello {{.name}}!" {
			t.Errorf("content: got %q", got.Content)
		}
	})

	t.Run("DeleteTemplate_CascadesVersions", func(t *testing.T) {
		tmpl, _ := store.CreateTemplate(ctx, &model.Template{Name: "to-delete"})
		_, _ = store.CreateVersion(ctx, tmpl.ID, "content {{.x}}", []string{"x"})

		if err := store.DeleteTemplate(ctx, tmpl.ID); err != nil {
			t.Fatalf("DeleteTemplate: %v", err)
		}
		// Versions should be gone (ON DELETE CASCADE)
		versions, _ := store.ListVersions(ctx, tmpl.ID)
		if len(versions) != 0 {
			t.Errorf("expected 0 versions after delete, got %d", len(versions))
		}
	})

	t.Run("UsageEventsAndCostReport", func(t *testing.T) {
		// Record events for two tenants
		events := []model.UsageEvent{
			{Tenant: "acme", App: "rag", Model: "gpt-4o-mini", PromptTokens: 1000, CompletionTokens: 200, CostUSD: 0.00027},
			{Tenant: "acme", App: "rag", Model: "gpt-4o-mini", PromptTokens: 2000, CompletionTokens: 400, CostUSD: 0.00054},
			{Tenant: "acme", App: "chat", Model: "gpt-4o", PromptTokens: 500, CompletionTokens: 100, CostUSD: 0.00225},
			{Tenant: "beta", App: "search", Model: "gpt-4o-mini", PromptTokens: 800, CompletionTokens: 150, CostUSD: 0.00021},
		}
		for _, e := range events {
			saved, err := store.RecordUsage(ctx, &e)
			if err != nil {
				t.Fatalf("RecordUsage: %v", err)
			}
			if saved.ID == "" {
				t.Fatal("expected non-empty ID")
			}
		}

		// ListUsageEvents with tenant filter
		from := time.Now().Add(-1 * time.Hour)
		to := time.Now().Add(1 * time.Hour)
		list, err := store.ListUsageEvents(ctx, "acme", "", "", from, to, 10, 0)
		if err != nil {
			t.Fatalf("ListUsageEvents: %v", err)
		}
		if len(list) != 3 {
			t.Errorf("acme events: got %d want 3", len(list))
		}

		// ListUsageEvents with model filter
		miniList, err := store.ListUsageEvents(ctx, "", "", "gpt-4o-mini", from, to, 10, 0)
		if err != nil {
			t.Fatalf("ListUsageEvents by model: %v", err)
		}
		if len(miniList) != 3 {
			t.Errorf("gpt-4o-mini events: got %d want 3", len(miniList))
		}

		// Cost report by model
		report, err := store.BuildCostReport(ctx, model.GroupByModel, from, to)
		if err != nil {
			t.Fatalf("BuildCostReport by model: %v", err)
		}
		if report.TotalEvents != 4 {
			t.Errorf("total_events: got %d want 4", report.TotalEvents)
		}
		if len(report.Groups) != 2 { // gpt-4o-mini and gpt-4o
			t.Errorf("groups: got %d want 2", len(report.Groups))
		}

		// Cost report by tenant
		tenantReport, err := store.BuildCostReport(ctx, model.GroupByTenant, from, to)
		if err != nil {
			t.Fatalf("BuildCostReport by tenant: %v", err)
		}
		if len(tenantReport.Groups) != 2 { // acme and beta
			t.Errorf("tenant groups: got %d want 2", len(tenantReport.Groups))
		}

		// Verify total cost is sum of all events
		wantTotal := 0.00027 + 0.00054 + 0.00225 + 0.00021
		if math.Abs(report.TotalCostUSD-wantTotal) > 0.000001 {
			t.Errorf("total cost: got %.6f want %.6f", report.TotalCostUSD, wantTotal)
		}
	})

	t.Run("CostReport_EmptyWindow", func(t *testing.T) {
		// Report for future window should return zero results
		future := time.Now().Add(24 * time.Hour)
		report, err := store.BuildCostReport(ctx, model.GroupByModel,
			future, future.Add(1*time.Hour))
		if err != nil {
			t.Fatalf("BuildCostReport empty: %v", err)
		}
		if report.TotalEvents != 0 {
			t.Errorf("expected 0 events, got %d", report.TotalEvents)
		}
		if len(report.Groups) != 0 {
			t.Errorf("expected 0 groups, got %d", len(report.Groups))
		}
	})
}
