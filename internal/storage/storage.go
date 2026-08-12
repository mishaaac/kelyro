// Package storage defines persistence boundaries for application state and
// secrets without exposing a database engine or credential backend.
package storage

// StateStore persists opaque application state by namespace and key.
type StateStore interface {
	Get(namespace, key string) (value []byte, found bool, err error)
	Set(namespace, key string, value []byte) error
	Delete(namespace, key string) error
}

// SecretStore provides named secret persistence without exposing where or how
// secrets are stored.
type SecretStore interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}
