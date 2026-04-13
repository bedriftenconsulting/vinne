package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

type TemplateData map[string]string

type TemplateRenderer struct {
	templates   map[string]*template.Template
	templateDir string
}

func NewTemplateRenderer(templateDir string) *TemplateRenderer {
	return &TemplateRenderer{
		templates:   make(map[string]*template.Template),
		templateDir: templateDir,
	}
}

func NewDefaultTemplateRenderer() *TemplateRenderer {
	return NewTemplateRenderer(DefaultTemplatePath)
}

func (tr *TemplateRenderer) LoadTemplate(name, filePath string) error {
	fullPath := filepath.Join(tr.templateDir, filePath)
	tmpl, err := template.ParseFiles(fullPath)
	if err != nil {
		return fmt.Errorf("failed to parse template %s: %w", name, err)
	}
	tr.templates[name] = tmpl
	return nil
}

func (tr *TemplateRenderer) LoadAllTemplates() error {
	for name, templ := range EmailTemplates {
		if err := tr.LoadTemplate(string(name), templ.Path); err != nil {
			return err
		}
	}

	for name, templ := range SMSTemplates {
		if err := tr.LoadTemplate(string(name), templ.Path); err != nil {
			return err
		}
	}

	for name, templ := range PushTemplates {
		if err := tr.LoadTemplate(string(name), templ.Path); err != nil {
			return err
		}
	}

	return nil
}

func (tr *TemplateRenderer) RenderTemplate(name string, data TemplateData) (string, error) {
	tmpl, exists := tr.templates[name]
	if !exists {
		return "", fmt.Errorf("template %s not found", name)
	}

	// Convert TemplateData (map[string]string) to a format that supports dot notation
	// Go templates can access map[string]any with dot notation, but not map[string]string
	templateVars := make(map[string]any, len(data))
	for k, v := range data {
		templateVars[k] = v
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, templateVars); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", name, err)
	}

	return buf.String(), nil
}

func RenderHTMLTemplate(templateContent string, placeholders map[string]string) string {
	result := templateContent
	for key, value := range placeholders {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		result = strings.ReplaceAll(result, placeholder, value)
	}
	return result
}

func ReplacePlaceholders(content string, data TemplateData) string {
	result := content
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		strValue := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, strValue)
	}
	return result
}

func LoadHTMLFromFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read template file %s: %w", filePath, err)
	}
	return string(content), nil
}

func ValidateTemplate(templateContent string) error {
	_, err := template.New("validation").Parse(templateContent)
	if err != nil {
		return fmt.Errorf("invalid template syntax: %w", err)
	}
	return nil
}

func ExtractPlaceholders(templateContent string) []string {
	var placeholders []string
	lines := strings.Split(templateContent, "\n")
	for _, line := range lines {
		start := 0
		for {
			idx := strings.Index(line[start:], "{{.")
			if idx == -1 {
				break
			}
			idx += start
			end := strings.Index(line[idx:], "}}")
			if end == -1 {
				break
			}
			end += idx
			placeholder := line[idx+3 : end]
			placeholders = append(placeholders, placeholder)
			start = end + 2
		}
	}
	return placeholders
}

func ConvertToPlainText(html string) string {
	html = strings.ReplaceAll(html, "<br>", "\n")
	html = strings.ReplaceAll(html, "<br/>", "\n")
	html = strings.ReplaceAll(html, "<br />", "\n")
	html = strings.ReplaceAll(html, "</p>", "\n\n")
	html = strings.ReplaceAll(html, "</div>", "\n")
	html = strings.ReplaceAll(html, "</h1>", "\n")
	html = strings.ReplaceAll(html, "</h2>", "\n")
	html = strings.ReplaceAll(html, "</h3>", "\n")
	html = strings.ReplaceAll(html, "</h4>", "\n")
	html = strings.ReplaceAll(html, "</h5>", "\n")
	html = strings.ReplaceAll(html, "</h6>", "\n")

	for strings.Contains(html, "<") {
		start := strings.Index(html, "<")
		end := strings.Index(html[start:], ">")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+1:]
	}

	lines := strings.Split(html, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

func MinifyHTML(html string) string {
	lines := strings.Split(html, "\n")
	var result []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "")
}
