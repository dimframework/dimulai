package dimulai

import (
	"context"
	"embed"
	"fmt"
	"strings"

	"github.com/atfromhome/goreus/pkg/templates"
	"github.com/dimframework/dim"
)

//go:embed templates/emails/layouts/*.html templates/emails/components/*.html templates/emails/*.html templates/emails/*.txt
var emailTemplates embed.FS

// EmailService handles email sending with templates
type EmailService struct {
	mailer    dim.Mailer
	config    *dim.EmailConfig
	baseData  dim.BaseEmailData
	templates *templates.Manager
}

// PasswordResetEmailData holds data for password reset email
type PasswordResetEmailData struct {
	dim.BaseEmailData
	UserName  string
	ResetURL  string
	ExpiresIn string
}

// NewEmailService creates a new email service
func NewEmailService(mailer dim.Mailer, config *dim.EmailConfig) (*EmailService, error) {
	// Custom helpers
	helpers := map[string]any{
		"dict": func(values ...interface{}) map[string]interface{} {
			d := make(map[string]interface{})
			for i := 0; i < len(values); i += 2 {
				if i+1 < len(values) {
					key, _ := values[i].(string)
					d[key] = values[i+1]
				}
			}
			return d
		},
		"alertBg": func(t string) string {
			m := map[string]string{
				"info": "#e3f2fd", "success": "#e8f5e9", "warning": "#fff3e0", "error": "#ffebee",
			}
			if c, ok := m[t]; ok {
				return c
			}
			return m["info"]
		},
		"alertBorder": func(t string) string {
			m := map[string]string{
				"info": "#1976d2", "success": "#388e3c", "warning": "#f57c00", "error": "#d32f2f",
			}
			if c, ok := m[t]; ok {
				return c
			}
			return m["info"]
		},
	}

	// Initialize goreus template manager with global components
	tmplManager, err := templates.New(templates.Config{
		FS:           emailTemplates,
		TemplatesDir: "templates/emails",
		Helpers:      helpers,
		GlobalComponents: []string{
			"layouts/*.html",
			"components/*.html",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to init template manager: %w", err)
	}

	return &EmailService{
		mailer:    mailer,
		config:    config,
		baseData:  dim.NewBaseEmailData(config),
		templates: tmplManager,
	}, nil
}

// send handles loading, rendering (HTML & Text), and sending the email.
func (s *EmailService) send(ctx context.Context, to []string, subject, baseName string, data interface{}) error {
	htmlFile := baseName + ".html"
	txtFile := baseName + ".txt"

	// 1. Load and Render HTML
	// Global components (layouts, etc) are already loaded via Config.GlobalComponents
	htmlTmpl, err := s.templates.LoadHTML(htmlFile)
	if err != nil {
		return fmt.Errorf("failed to load HTML template %s: %w", htmlFile, err)
	}
	htmlContent, err := s.templates.RenderHTML(htmlTmpl, htmlFile, data)
	if err != nil {
		return fmt.Errorf("failed to render HTML template %s: %w", htmlFile, err)
	}

	// 2. Load and Render Plain Text (Optional)
	var txtContent string
	txtTmpl, err := s.templates.LoadPlaintext(txtFile)
	if err == nil {
		// Use filename as template name for plaintext
		txtContent, _ = s.templates.RenderPlaintext(txtTmpl, txtFile, data)
	}

	// 3. Create and Send Message
	msg := dim.NewMailMessage(to, subject)
	msg.HTML = htmlContent
	msg.PlainText = txtContent

	return s.mailer.Send(ctx, msg)
}

// SendPasswordReset sends a password reset email
func (s *EmailService) SendPasswordReset(ctx context.Context, email, userName, token string) error {
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", strings.TrimRight(s.config.BaseURL, "/"), token)

	data := PasswordResetEmailData{
		BaseEmailData: s.baseData,
		UserName:      userName,
		ResetURL:      resetURL,
		ExpiresIn:     "1 hour",
	}

	return s.send(ctx, []string{email}, "Reset Your Password", "password_reset", data)
}
