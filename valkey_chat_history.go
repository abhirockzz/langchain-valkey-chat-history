package main

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
)

// ValkeyMessageHistory is a struct that stores chat messages.
type ValkeyMessageHistory struct {
	sessionID  string
	sessionTTL int // ttl in seconds
	client     *redis.Client
}

func New(sessionID string, sessionTTL int, client *redis.Client) (*ValkeyMessageHistory, error) {

	history := &ValkeyMessageHistory{
		sessionID:  sessionID,
		sessionTTL: sessionTTL,
		client:     client}

	return history, nil
}

// Statically assert that ValkeyMessageHistory implement the chat message history interface.
var _ schema.ChatMessageHistory = &ValkeyMessageHistory{}

func (h ValkeyMessageHistory) AddMessage(ctx context.Context, message llms.ChatMessage) error {
	return h.addMessage(context.Background(), lcMessageToValkeyMessage(message))
}

// AddUserMessage adds an user to the chat message history.
func (h ValkeyMessageHistory) AddUserMessage(_ context.Context, text string) error {
	return h.addMessage(context.Background(), ValkeyChatMessage{Type: "human", Content: text})
}

func (h ValkeyMessageHistory) AddAIMessage(_ context.Context, text string) error {
	return h.addMessage(context.Background(), ValkeyChatMessage{Type: "ai", Content: text})
}

func (h ValkeyMessageHistory) Clear(_ context.Context) error {
	return h.client.Del(context.Background(), h.sessionID).Err()
}

func (h ValkeyMessageHistory) SetMessages(ctx context.Context, messages []llms.ChatMessage) error {
	for _, message := range messages {
		err := h.AddMessage(context.Background(), message)

		if err != nil {
			return err
		}
	}

	return nil
}

// Messages returns all messages stored.
func (h ValkeyMessageHistory) Messages(ctx context.Context) ([]llms.ChatMessage, error) {
	messages, err := h.client.LRange(context.Background(), h.sessionID, 0, -1).Result()

	if err != nil {
		return nil, err
	}

	var chatMessages []llms.ChatMessage

	for _, m := range messages {
		var message ValkeyChatMessage

		err := json.Unmarshal([]byte(m), &message)
		if err != nil {
			return nil, err
		}

		var llmChatMessage llms.ChatMessage

		if message.Type == "human" {
			llmChatMessage = llms.HumanChatMessage{Content: message.Content}
		} else if message.Type == "ai" {
			llmChatMessage = llms.AIChatMessage{Content: message.Content}
		}

		chatMessages = append(chatMessages, llmChatMessage)
	}

	return chatMessages, nil
}

func (h *ValkeyMessageHistory) addMessage(_ context.Context, message ValkeyChatMessage) error {

	msg, err := json.Marshal(message)
	if err != nil {
		return err
	}

	pipeline := h.client.Pipeline()

	err = pipeline.LPush(context.Background(), h.sessionID, string(msg)).Err()

	if err != nil {
		return err
	}

	err = pipeline.Expire(context.Background(), h.sessionID, time.Duration(h.sessionTTL*int(time.Second))).Err()

	if err != nil {
		return err
	}

	_, err = pipeline.Exec(context.Background())

	if err != nil {
		return err
	}

	return nil
}

func lcMessageToValkeyMessage(message llms.ChatMessage) ValkeyChatMessage {
	return ValkeyChatMessage{
		Type:    string(message.GetType()),
		Content: message.GetContent(),
	}
}

type ValkeyChatMessage struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}
