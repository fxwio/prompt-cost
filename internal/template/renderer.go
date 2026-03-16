// Package template provides prompt template rendering with variable injection.
// Templates use Go text/template syntax: {{.variable_name}}
package template

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"text/template"
)

// varPattern matches {{.identifier}} expressions (with optional whitespace).
var varPattern = regexp.MustCompile(`\{\{\s*\.(\w+)\s*\}\}`)

// ExtractVariables returns all unique variable names referenced in content.
// For example: "Hello {{.name}}, you are {{.age}} years old" → ["name", "age"]
func ExtractVariables(content string) []string {
	matches := varPattern.FindAllStringSubmatch(content, -1)
	seen := map[string]bool{}
	out := []string{}
	for _, m := range matches {
		if !seen[m[1]] {
			seen[m[1]] = true
			out = append(out, m[1])
		}
	}
	return out
}

// Render executes the template content with the provided variables.
// Returns an error if a required variable is missing or the template is invalid.
func Render(content string, variables map[string]string) (string, error) {
	if err := validateVariables(content, variables); err != nil {
		return "", err
	}

	// Convert map[string]string to map[string]any for text/template
	data := make(map[string]any, len(variables))
	for k, v := range variables {
		data[k] = v
	}

	tmpl, err := template.New("prompt").Option("missingkey=error").Parse(content)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}
	return buf.String(), nil
}

// validateVariables checks that all required variables are provided.
func validateVariables(content string, provided map[string]string) error {
	required := ExtractVariables(content)
	var missing []string
	for _, v := range required {
		if _, ok := provided[v]; !ok {
			missing = append(missing, v)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required variables: %s", strings.Join(missing, ", "))
	}
	return nil
}
