package types

// Message types used throughout the application
const (
	MessageTypeSystem         = "system"
	MessageTypeEncrypted      = "encrypted_message"
	MessageTypePublicKeyShare = "public_key_share"
	MessageTypeRequestKeys    = "request_keys"
	MessageTypeUserInfo       = "user_info"
	MessageTypeKeyExchange    = "key_exchange"
)

// Session cookie name
const SessionCookieName = "chapp_session"

// System sender name
const SystemSender = "System"
