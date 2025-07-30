package types

import "github.com/gorilla/websocket"

// ClientInterface defines the common interface for both client and server clients
type ClientInterface interface {
	GetUsername() string
	GetConnection() *websocket.Conn
}

// BaseClient contains the common fields shared between client and server clients
type BaseClient struct {
	Conn     *websocket.Conn
	Username string
}

// GetUsername returns the client's username
func (c *BaseClient) GetUsername() string {
	return c.Username
}

// GetConnection returns the client's WebSocket connection
func (c *BaseClient) GetConnection() *websocket.Conn {
	return c.Conn
}
