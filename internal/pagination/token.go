// Package pagination provides an opaque, token-based cursor shared by every
// paginated API the registry exposes. Callers never see or construct the
// underlying offset directly: they pass whatever token the previous response
// handed back, which keeps the wire contract stable even if the server-side
// pagination strategy changes later (e.g. a move from offset to keyset
// pagination).
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

// ErrInvalidToken is returned when a page token fails to decode. Callers
// should surface this as an invalid-argument style error to the client.
var ErrInvalidToken = errors.New("invalid page token")

type token struct {
	Offset uint32 `json:"offset"`
}

// DecodeOffset decodes a page token into the offset it represents. An empty
// token decodes to offset 0, i.e. the first page.
func DecodeOffset(pageToken string) (uint32, error) {
	if pageToken == "" {
		return 0, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(pageToken)
	if err != nil {
		return 0, ErrInvalidToken
	}

	var t token
	if err := json.Unmarshal(raw, &t); err != nil {
		return 0, ErrInvalidToken
	}

	return t.Offset, nil
}

// NextToken returns the opaque token for the page following one that started
// at offset and returned `returned` results out of `total` matching rows. It
// returns "" once there is nothing left to page through.
func NextToken(offset uint32, returned int, total int64) string {
	if returned <= 0 {
		return ""
	}

	next := uint64(offset) + uint64(returned)
	if next >= uint64(total) {
		return ""
	}

	raw, _ := json.Marshal(token{Offset: uint32(next)})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// ClampPageSize normalizes a requested page size against a default and max,
// treating a non-positive size as "use the default".
func ClampPageSize(size int, def, max uint32) uint32 {
	if size <= 0 {
		return def
	}
	if uint32(size) > max {
		return max
	}
	return uint32(size)
}
