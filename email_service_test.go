package dimulai

import (
	"context"
	"strings"
	"testing"

	"github.com/dimframework/dim"
)

// MockMailer captures sent messages
type MockMailer struct {
	SentMessages []*dim.MailMessage
}

func (m *MockMailer) Send(ctx context.Context, msg *dim.MailMessage) error {
	m.SentMessages = append(m.SentMessages, msg)
	return nil
}

func TestEmailService_SendPasswordReset(t *testing.T) {
	// Setup dependencies
	mockMailer := &MockMailer{}
	cfg := &dim.EmailConfig{
		AppName: "Test App",
		BaseURL: "http://localhost:3000",
		From:    "test@example.com",
	}

	service, err := NewEmailService(mockMailer, cfg)
	if err != nil {
		t.Fatalf("failed to create email service: %v", err)
	}

	// Test data
	email := "user@example.com"
	userName := "John Doe"
	token := "reset-token-123"

	// Execute
	err = service.SendPasswordReset(context.Background(), email, userName, token)
	if err != nil {
		t.Fatalf("SendPasswordReset failed: %v", err)
	}

	// Verify
	if len(mockMailer.SentMessages) != 1 {
		t.Fatalf("expected 1 message sent, got %d", len(mockMailer.SentMessages))
	}

	msg := mockMailer.SentMessages[0]
	if msg.To[0] != email {
		t.Errorf("expected recipient %s, got %s", email, msg.To[0])
	}
	if msg.Subject != "Reset Your Password" {
		t.Errorf("expected subject 'Reset Your Password', got %s", msg.Subject)
	}

	// Verify content rendering (simple check)
	if !strings.Contains(msg.HTML, "John Doe") {
		t.Error("HTML content missing user name")
	}
	if !strings.Contains(msg.HTML, token) {
		t.Error("HTML content missing token")
	}
	// Verify Plain Text (loaded from .txt template)
	if !strings.Contains(msg.PlainText, "John Doe") {
		t.Error("Plain text content missing user name")
	}
}
