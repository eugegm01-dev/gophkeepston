// internal/sync/policy.go
package sync

import (
	"fmt"
)

type Policy string

const (
	PolicyLastWriteWins       Policy = "last-write-wins"
	PolicyServerAuthoritative Policy = "server-authoritative"
	PolicyClientAuthoritative Policy = "client-authoritative"
)

type ConflictResolver interface {
	Resolve(local, remote *Entry) (*Entry, error)
}

type LastWriteWins struct{}

func (l LastWriteWins) Resolve(local, remote *Entry) (*Entry, error) {
	if remote.UpdatedAt > local.UpdatedAt {
		return remote, nil
	}
	return local, nil
}

// Entry — упрощённая структура для сравнения (можно расширить)
type Entry struct {
	ID            string
	Version       int64
	EncryptedData []byte
	UpdatedAt     int64
}

func NewResolver(policy string) (ConflictResolver, error) {
	switch Policy(policy) {
	case PolicyLastWriteWins, "":
		return LastWriteWins{}, nil
	case PolicyServerAuthoritative:
		return serverAuthoritative{}, nil
	case PolicyClientAuthoritative:
		return clientAuthoritative{}, nil
	default:
		return nil, fmt.Errorf("unknown sync policy: %s", policy)
	}
}

type serverAuthoritative struct{}

func (s serverAuthoritative) Resolve(_, remote *Entry) (*Entry, error) { return remote, nil }

type clientAuthoritative struct{}

func (c clientAuthoritative) Resolve(local, _ *Entry) (*Entry, error) { return local, nil }
