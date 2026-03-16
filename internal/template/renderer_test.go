package template

import (
	"strings"
	"testing"
)

func TestExtractVariables(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    []string
	}{
		{
			name:    "single variable",
			content: "Hello {{.name}}",
			want:    []string{"name"},
		},
		{
			name:    "multiple variables",
			content: "Hello {{.name}}, you are {{.age}} years old and live in {{.city}}.",
			want:    []string{"name", "age", "city"},
		},
		{
			name:    "duplicate variables deduplicated",
			content: "{{.topic}} is important. Tell me about {{.topic}}.",
			want:    []string{"topic"},
		},
		{
			name:    "whitespace in braces",
			content: "Hello {{ .name }} and {{ .city }}",
			want:    []string{"name", "city"},
		},
		{
			name:    "no variables",
			content: "Hello world, no variables here.",
			want:    []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractVariables(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("got[%d]=%q, want %q", i, v, tt.want[i])
				}
			}
		})
	}
}

func TestRender_Success(t *testing.T) {
	content := "You are a helpful assistant. Answer the following question about {{.topic}}: {{.question}}"
	vars := map[string]string{
		"topic":    "Go programming",
		"question": "What is a goroutine?",
	}
	got, err := Render(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Go programming") {
		t.Errorf("rendered output missing topic: %q", got)
	}
	if !strings.Contains(got, "What is a goroutine?") {
		t.Errorf("rendered output missing question: %q", got)
	}
}

func TestRender_MissingVariable(t *testing.T) {
	content := "Hello {{.name}}, you are {{.age}} years old."
	vars := map[string]string{"name": "Alice"} // missing "age"
	_, err := Render(content, vars)
	if err == nil {
		t.Fatal("expected error for missing variable, got nil")
	}
	if !strings.Contains(err.Error(), "age") {
		t.Errorf("error should mention missing var 'age', got: %v", err)
	}
}

func TestRender_NoVariables(t *testing.T) {
	content := "This is a static prompt with no variables."
	got, err := Render(content, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != content {
		t.Errorf("got %q, want %q", got, content)
	}
}

func TestRender_ExtraVariablesIgnored(t *testing.T) {
	content := "Hello {{.name}}!"
	vars := map[string]string{"name": "Bob", "extra": "ignored"}
	got, err := Render(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "Hello Bob!" {
		t.Errorf("got %q, want %q", got, "Hello Bob!")
	}
}

func TestRender_MultilineTemplate(t *testing.T) {
	content := `You are an expert in {{.domain}}.

User question: {{.question}}

Please provide a detailed answer in {{.language}}.`
	vars := map[string]string{
		"domain":   "cloud computing",
		"question": "What is Kubernetes?",
		"language": "English",
	}
	got, err := Render(content, vars)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "cloud computing") {
		t.Errorf("missing domain in output: %q", got)
	}
}

func TestRender_InvalidTemplateSyntax(t *testing.T) {
	content := "Hello {{.name" // unclosed brace
	_, err := Render(content, map[string]string{"name": "Bob"})
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
}
