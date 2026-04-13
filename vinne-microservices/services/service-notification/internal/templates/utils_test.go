package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateRenderer(t *testing.T) {
	tempDir := t.TempDir()
	templatePath := filepath.Join(tempDir, "test.html")

	templateContent := `<h1>Hello {{.Name}}!</h1><p>Welcome to {{.Company}}.</p>`
	err := os.WriteFile(templatePath, []byte(templateContent), 0644)
	require.NoError(t, err)

	renderer := NewTemplateRenderer(tempDir)
	err = renderer.LoadTemplate("test", "test.html")
	require.NoError(t, err)

	data := TemplateData{
		"Name":    "John",
		"Company": "RAND Lottery",
	}

	result, err := renderer.RenderTemplate("test", data)
	require.NoError(t, err)
	assert.Contains(t, result, "Hello John!")
	assert.Contains(t, result, "RAND Lottery")
}

func TestNewDefaultTemplateRenderer(t *testing.T) {
	renderer := NewDefaultTemplateRenderer()
	assert.Equal(t, DefaultTemplatePath, renderer.templateDir)
}

func TestReplacePlaceholders(t *testing.T) {
	content := "Hello {{Name}}, welcome to {{Company}}!"
	data := TemplateData{
		"Name":    "John",
		"Company": "RAND Lottery",
	}

	result := ReplacePlaceholders(content, data)
	assert.Equal(t, "Hello John, welcome to RAND Lottery!", result)
}

func TestRenderHTMLTemplate(t *testing.T) {
	template := "<h1>Welcome {{.FirstName}}!</h1><p>Email: {{.Email}}</p>"
	placeholders := map[string]string{
		"FirstName": "John",
		"Email":     "john@example.com",
	}

	result := RenderHTMLTemplate(template, placeholders)
	assert.Contains(t, result, "Welcome John!")
	assert.Contains(t, result, "john@example.com")
}

func TestExtractPlaceholders(t *testing.T) {
	template := "<h1>Welcome {{.FirstName}}!</h1><p>Email: {{.Email}}</p><p>Company: {{.Company}}</p>"
	placeholders := ExtractPlaceholders(template)

	assert.Contains(t, placeholders, "FirstName")
	assert.Contains(t, placeholders, "Email")
	assert.Contains(t, placeholders, "Company")
	assert.Len(t, placeholders, 3)
}

func TestConvertToPlainText(t *testing.T) {
	html := "<h1>Welcome!</h1><p>Thank you for joining us.</p><br/><div>Best regards</div>"
	plainText := ConvertToPlainText(html)

	assert.Contains(t, plainText, "Welcome!")
	assert.Contains(t, plainText, "Thank you for joining us.")
	assert.Contains(t, plainText, "Best regards")
	assert.NotContains(t, plainText, "<h1>")
	assert.NotContains(t, plainText, "<p>")
}

func TestValidateTemplate(t *testing.T) {
	validTemplate := "<h1>Hello {{.Name}}!</h1>"
	err := ValidateTemplate(validTemplate)
	assert.NoError(t, err)

	invalidTemplate := "<h1>Hello {{.Name}!</h1>"
	err = ValidateTemplate(invalidTemplate)
	assert.Error(t, err)
}

func TestMinifyHTML(t *testing.T) {
	html := `
	<h1>Welcome!</h1>
	
	<p>Thank you for joining us.</p>
	
	`

	minified := MinifyHTML(html)
	assert.Equal(t, "<h1>Welcome!</h1><p>Thank you for joining us.</p>", minified)
}
