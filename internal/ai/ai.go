// Package ai abstracts the LLM provider so it can be swapped (DeepSeek now,
// others later) without touching feature code.
package ai

import "context"

// Message is a single chat message.
type Message struct {
	Role    string // "system" | "user" | "assistant"
	Content string
}

// Provider is an LLM chat provider.
type Provider interface {
	// Chat sends messages and returns the assistant reply.
	Chat(ctx context.Context, messages []Message) (string, error)
	// Name identifies the provider (for logging/diagnostics).
	Name() string
}
