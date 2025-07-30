package types

// Message represents a chat message (server can't read encrypted content)
type Message struct {
	Type      string `json:"type"`
	Content   string `json:"content"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient,omitempty"`
	Timestamp int64  `json:"timestamp"`
}
