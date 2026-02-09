// Package mqtt provides a notification service for MQTT message brokers.
package mqtt

import (
	"context"

	"github.com/eclipse/paho.golang/paho"
)

// ConnectionManager defines the interface for MQTT connection management.
type ConnectionManager interface {
	// AwaitConnection waits for the MQTT connection to be established.
	// It blocks until the connection is ready or the context is cancelled.
	AwaitConnection(ctx context.Context) error

	// Publish sends an MQTT message to the broker.
	// Returns a PublishResponse containing the reason code and any properties,
	// or an error if the publish operation fails.
	Publish(ctx context.Context, publish *paho.Publish) (*paho.PublishResponse, error)

	// Disconnect gracefully closes the MQTT connection.
	// It sends a DISCONNECT packet to the broker and waits for the operation
	// to complete or the context to be cancelled.
	Disconnect(ctx context.Context) error
}
