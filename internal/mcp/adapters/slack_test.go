package adapters

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockSlackClient implements SlackClient for testing
type MockSlackClient struct {
	channels    []SlackChannel
	users       []SlackUser
	messages    []SlackMessage
	files       []SlackFile
	shouldError bool
}

func NewMockSlackClient() *MockSlackClient {
	return &MockSlackClient{
		channels: []SlackChannel{
			{ID: "C001", Name: "general", IsPrivate: false, IsArchived: false, IsMember: true, NumMembers: 50, Topic: "General discussion", Purpose: "Company-wide announcements"},
			{ID: "C002", Name: "engineering", IsPrivate: false, IsArchived: false, IsMember: true, NumMembers: 20, Topic: "Engineering team", Purpose: "Technical discussions"},
			{ID: "C003", Name: "private-team", IsPrivate: true, IsArchived: false, IsMember: true, NumMembers: 5, Topic: "Private channel", Purpose: "Team discussions"},
		},
		users: []SlackUser{
			{ID: "U001", Name: "alice", RealName: "Alice Smith", Email: "alice@example.com", IsBot: false, IsAdmin: true, Status: "active"},
			{ID: "U002", Name: "bob", RealName: "Bob Jones", Email: "bob@example.com", IsBot: false, IsAdmin: false, Status: "away"},
			{ID: "U003", Name: "slack-bot", RealName: "Slack Bot", IsBot: true, IsAdmin: false},
		},
		messages: []SlackMessage{
			{TS: "1234567890.000001", Text: "Hello everyone!", User: "U001", Channel: "C001"},
			{TS: "1234567890.000002", Text: "Hi Alice!", User: "U002", Channel: "C001", ThreadTS: "1234567890.000001"},
		},
		files: []SlackFile{
			{ID: "F001", Name: "report.pdf", URL: "https://files.slack.com/files-pri/F001/report.pdf", Size: 1024000, Mimetype: "application/pdf"},
		},
	}
}

func (m *MockSlackClient) SetError(shouldError bool) {
	m.shouldError = shouldError
}

func (m *MockSlackClient) PostMessage(ctx context.Context, channel, text string, options MessageOptions) (string, error) {
	if m.shouldError {
		return "", assert.AnError
	}
	return "1234567890.000100", nil
}

func (m *MockSlackClient) UpdateMessage(ctx context.Context, channel, ts, text string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) DeleteMessage(ctx context.Context, channel, ts string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) GetMessage(ctx context.Context, channel, ts string) (*SlackMessage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, msg := range m.messages {
		if msg.TS == ts && msg.Channel == channel {
			return &msg, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockSlackClient) ListChannels(ctx context.Context, types string, limit int) ([]SlackChannel, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if limit > len(m.channels) {
		return m.channels, nil
	}
	return m.channels[:limit], nil
}

func (m *MockSlackClient) GetChannel(ctx context.Context, channelID string) (*SlackChannel, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, ch := range m.channels {
		if ch.ID == channelID {
			return &ch, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockSlackClient) CreateChannel(ctx context.Context, name string, isPrivate bool) (*SlackChannel, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &SlackChannel{
		ID:        "C999",
		Name:      name,
		IsPrivate: isPrivate,
	}, nil
}

func (m *MockSlackClient) ArchiveChannel(ctx context.Context, channelID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) JoinChannel(ctx context.Context, channelID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) LeaveChannel(ctx context.Context, channelID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) InviteToChannel(ctx context.Context, channelID, userID string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) ListUsers(ctx context.Context, limit int) ([]SlackUser, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	if limit > len(m.users) {
		return m.users, nil
	}
	return m.users[:limit], nil
}

func (m *MockSlackClient) GetUser(ctx context.Context, userID string) (*SlackUser, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	for _, u := range m.users {
		if u.ID == userID {
			return &u, nil
		}
	}
	return nil, assert.AnError
}

func (m *MockSlackClient) AddReaction(ctx context.Context, channel, ts, emoji string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) RemoveReaction(ctx context.Context, channel, ts, emoji string) error {
	if m.shouldError {
		return assert.AnError
	}
	return nil
}

func (m *MockSlackClient) UploadFile(ctx context.Context, channels []string, filename, content string) (*SlackFile, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return &SlackFile{
		ID:       "F999",
		Name:     filename,
		URL:      "https://files.slack.com/files-pri/F999/" + filename,
		Size:     len(content),
		Mimetype: "text/plain",
	}, nil
}

func (m *MockSlackClient) SearchMessages(ctx context.Context, query string, count int) ([]SlackMessage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	return m.messages, nil
}

func (m *MockSlackClient) GetConversationHistory(ctx context.Context, channel string, limit int) ([]SlackMessage, error) {
	if m.shouldError {
		return nil, assert.AnError
	}
	var result []SlackMessage
	for _, msg := range m.messages {
		if msg.Channel == channel {
			result = append(result, msg)
		}
	}
	return result, nil
}

// Tests

func TestDefaultSlackConfig(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()

	assert.Equal(t, 30*time.Second, config.Timeout)
}

func TestNewSlackAdapter(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	assert.NotNil(t, adapter)

	info := adapter.GetServerInfo()
	assert.Equal(t, "slack", info.Name)
	assert.Equal(t, "1.0.0", info.Version)
}

func TestSlackAdapter_ListTools(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	tools := adapter.ListTools()

	assert.NotEmpty(t, tools)
	toolNames := make([]string, len(tools))
	for i, tool := range tools {
		toolNames[i] = tool.Name
	}
	assert.Contains(t, toolNames, "slack_post_message")
	assert.Contains(t, toolNames, "slack_update_message")
	assert.Contains(t, toolNames, "slack_delete_message")
	assert.Contains(t, toolNames, "slack_list_channels")
	assert.Contains(t, toolNames, "slack_create_channel")
	assert.Contains(t, toolNames, "slack_list_users")
	assert.Contains(t, toolNames, "slack_add_reaction")
	assert.Contains(t, toolNames, "slack_upload_file")
	assert.Contains(t, toolNames, "slack_search_messages")
	assert.Contains(t, toolNames, "slack_get_history")
}

func TestSlackAdapter_PostMessage(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_post_message", map[string]interface{}{
		"channel": "C001",
		"text":    "Hello, World!",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.IsError)
}

func TestSlackAdapter_PostMessageWithThread(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_post_message", map[string]interface{}{
		"channel":   "C001",
		"text":      "This is a reply",
		"thread_ts": "1234567890.000001",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_UpdateMessage(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_update_message", map[string]interface{}{
		"channel": "C001",
		"ts":      "1234567890.000001",
		"text":    "Updated message",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_DeleteMessage(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_delete_message", map[string]interface{}{
		"channel": "C001",
		"ts":      "1234567890.000001",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_ListChannels(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_list_channels", map[string]interface{}{
		"types": "public_channel",
		"limit": 100,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_CreateChannel(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_create_channel", map[string]interface{}{
		"name":       "new-channel",
		"is_private": false,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_ArchiveChannel(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_archive_channel", map[string]interface{}{
		"channel": "C001",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_InviteToChannel(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_invite_to_channel", map[string]interface{}{
		"channel": "C002",
		"user":    "U001",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_ListUsers(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_list_users", map[string]interface{}{
		"limit": 100,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_AddReaction(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_add_reaction", map[string]interface{}{
		"channel": "C001",
		"ts":      "1234567890.000001",
		"emoji":   "thumbsup",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_UploadFile(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_upload_file", map[string]interface{}{
		"channels": []interface{}{"C001", "C002"},
		"filename": "test.txt",
		"content":  "File content here",
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_SearchMessages(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_search_messages", map[string]interface{}{
		"query": "hello",
		"count": 20,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_GetHistory(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_get_history", map[string]interface{}{
		"channel": "C001",
		"limit":   50,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func TestSlackAdapter_InvalidTool(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	_, err := adapter.CallTool(ctx, "invalid_tool", map[string]interface{}{})

	assert.Error(t, err)
}

func TestSlackAdapter_ErrorHandling(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	client.SetError(true)
	adapter := NewSlackAdapter(config, client)

	ctx := context.Background()
	result, err := adapter.CallTool(ctx, "slack_list_channels", map[string]interface{}{})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.IsError)
}

// Type tests

func TestSlackMessageTypes(t *testing.T) {
	t.Parallel()
	message := SlackMessage{
		TS:       "1234567890.000001",
		Text:     "Hello, everyone!",
		User:     "U001",
		Channel:  "C001",
		ThreadTS: "",
		Reactions: []Reaction{
			{Name: "thumbsup", Count: 5, Users: []string{"U001", "U002"}},
		},
	}

	assert.Equal(t, "1234567890.000001", message.TS)
	assert.Equal(t, "Hello, everyone!", message.Text)
	assert.Equal(t, "U001", message.User)
	assert.Len(t, message.Reactions, 1)
}

func TestSlackChannelTypes(t *testing.T) {
	t.Parallel()
	channel := SlackChannel{
		ID:         "C123",
		Name:       "dev-team",
		IsPrivate:  true,
		IsArchived: false,
		IsMember:   true,
		NumMembers: 15,
		Topic:      "Development discussions",
		Purpose:    "For the dev team",
	}

	assert.Equal(t, "C123", channel.ID)
	assert.Equal(t, "dev-team", channel.Name)
	assert.True(t, channel.IsPrivate)
	assert.False(t, channel.IsArchived)
	assert.Equal(t, 15, channel.NumMembers)
}

func TestSlackUserTypes(t *testing.T) {
	t.Parallel()
	user := SlackUser{
		ID:       "U456",
		Name:     "jsmith",
		RealName: "John Smith",
		Email:    "john@example.com",
		IsBot:    false,
		IsAdmin:  true,
		Status:   "active",
	}

	assert.Equal(t, "U456", user.ID)
	assert.Equal(t, "jsmith", user.Name)
	assert.Equal(t, "John Smith", user.RealName)
	assert.False(t, user.IsBot)
	assert.True(t, user.IsAdmin)
}

func TestSlackFileTypes(t *testing.T) {
	t.Parallel()
	file := SlackFile{
		ID:       "F789",
		Name:     "document.pdf",
		URL:      "https://files.slack.com/files-pri/F789/document.pdf",
		Size:     2048000,
		Mimetype: "application/pdf",
	}

	assert.Equal(t, "F789", file.ID)
	assert.Equal(t, "document.pdf", file.Name)
	assert.Equal(t, 2048000, file.Size)
	assert.Equal(t, "application/pdf", file.Mimetype)
}

func TestReactionTypes(t *testing.T) {
	t.Parallel()
	reaction := Reaction{
		Name:  "rocket",
		Count: 3,
		Users: []string{"U001", "U002", "U003"},
	}

	assert.Equal(t, "rocket", reaction.Name)
	assert.Equal(t, 3, reaction.Count)
	assert.Len(t, reaction.Users, 3)
}

func TestMessageOptionsTypes(t *testing.T) {
	t.Parallel()
	options := MessageOptions{
		ThreadTS:       "1234567890.000001",
		ReplyBroadcast: true,
		Blocks: []Block{
			{Type: "section", Text: &TextObject{Type: "mrkdwn", Text: "*Bold text*"}},
		},
		Attachments: []Attachment{
			{Color: "#ff0000", Title: "Alert", Text: "This is important"},
		},
		Unfurl: true,
	}

	assert.Equal(t, "1234567890.000001", options.ThreadTS)
	assert.True(t, options.ReplyBroadcast)
	assert.Len(t, options.Blocks, 1)
	assert.Len(t, options.Attachments, 1)
}

func TestBlockTypes(t *testing.T) {
	t.Parallel()
	block := Block{
		Type: "section",
		Text: &TextObject{
			Type: "mrkdwn",
			Text: "This is *bold* text",
		},
	}

	assert.Equal(t, "section", block.Type)
	assert.NotNil(t, block.Text)
	assert.Equal(t, "mrkdwn", block.Text.Type)
}

func TestAttachmentTypes(t *testing.T) {
	t.Parallel()
	attachment := Attachment{
		Color:  "good",
		Title:  "Success",
		Text:   "Operation completed successfully",
		Footer: "Slack API",
	}

	assert.Equal(t, "good", attachment.Color)
	assert.Equal(t, "Success", attachment.Title)
	assert.Equal(t, "Operation completed successfully", attachment.Text)
}

func TestSlackConfigTypes(t *testing.T) {
	t.Parallel()
	config := SlackConfig{
		BotToken: "xoxb-xxxxxxxxxxxx",
		AppToken: "xapp-xxxxxxxxxxxx",
		Timeout:  60 * time.Second,
		TeamID:   "T12345678",
	}

	assert.NotEmpty(t, config.BotToken)
	assert.NotEmpty(t, config.AppToken)
	assert.Equal(t, 60*time.Second, config.Timeout)
	assert.Equal(t, "T12345678", config.TeamID)
}

func TestSlackAdapter_GetServerInfoCapabilities(t *testing.T) {
	t.Parallel()
	config := DefaultSlackConfig()
	client := NewMockSlackClient()
	adapter := NewSlackAdapter(config, client)

	info := adapter.GetServerInfo()
	assert.Contains(t, info.Capabilities, "messaging")
	assert.Contains(t, info.Capabilities, "channels")
	assert.Contains(t, info.Capabilities, "users")
	assert.Contains(t, info.Capabilities, "files")
	assert.Contains(t, info.Capabilities, "reactions")
	assert.Contains(t, info.Capabilities, "search")
}

func TestTruncateFunction(t *testing.T) {
	t.Parallel()
	// Test short string (no truncation)
	short := "Hello"
	assert.Equal(t, "Hello", truncate(short, 10))

	// Test exact length
	exact := "Hello"
	assert.Equal(t, "Hello", truncate(exact, 5))

	// Test long string (truncation)
	long := "Hello, World! This is a long message."
	result := truncate(long, 10)
	assert.Equal(t, "Hello, Wor...", result)
}
