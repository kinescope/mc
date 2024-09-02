package mc

import (
	"errors"
	"fmt"

	"github.com/kinescope/mc/protocol"
)

var (
	ErrCacheMiss        = errors.New("memcache: cache miss")
	ErrNotStored        = errors.New("memcache: item not stored")
	ErrNoServers        = errors.New("memcache: no servers configured or available")
	ErrBadIncrDec       = errors.New("memcache: incr or decr on non-numeric value")
	ErrCASConflict      = errors.New("memcache: compare-and-swap conflict")
	ErrServerError      = errors.New("memcache: server error")
	ErrMalformedKey     = errors.New("memcache: key is too long or contains invalid characters")
	ErrAlreadyExists    = errors.New("memcache: item already exists")
	ErrValueTooLarge    = errors.New("memcache: value too large")
	ErrInvalidArguments = errors.New("memcache: invalid arguments")
)

func checkError(err error) error {
	switch e := err.(type) {
	case protocol.Status:
		switch e {
		case protocol.StatusKeyExists:
			return ErrAlreadyExists
		case protocol.StatusKeyNotFound:
			return ErrCacheMiss
		case protocol.StatusItemNotStored:
			return ErrNotStored
		case protocol.StatusInternalError:
			return ErrServerError
		case protocol.StatusInvalidArguments:
			return ErrInvalidArguments
		case protocol.StatusIncrDecrOnNonNumericValue:
			return ErrBadIncrDec
		case protocol.StatusValueTooLarge:
			return ErrValueTooLarge
		default:
			return fmt.Errorf("memcache: status=%d", e)
		}
	}
	return err
}
