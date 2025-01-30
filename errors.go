package mc

import (
	"errors"
)

var (
	ErrCacheMiss                = errors.New("memcache: cache miss")
	ErrNotStored                = errors.New("memcache: item not stored")
	ErrCASConflict              = errors.New("memcache: compare-and-swap conflict")
	ErrMalformedKey             = errors.New("memcache: key is too long or contains invalid characters")
	ErrNoServers                = errors.New("memcache: no servers configured or available")
	ErrNonexistentCommandName   = errors.New("memcache: nonexistent command name")
	ErrUnsupportedServerVersion = errors.New("memcache: unsupported server version ( < 1.6.14 )")
	ErrBadIncrDec               = errors.New("memcache: cannot increment or decrement non-numeric value")
	ErrCorruptGetResultRead     = errors.New("memcache: corrupt get result read")
	//ErrServerError              = errors.New("memcache: server error")
	//ErrAlreadyExists = errors.New("memcache: item already exists")
	//ErrValueTooLarge            = errors.New("memcache: value too large")
	//ErrInvalidArguments         = errors.New("memcache: invalid arguments")

)

type (
	ClientError struct {
		Message string
	}
	ServerError struct {
		Message string
	}
)

func (c *ClientError) Error() string { return "memcache [client]: " + c.Message }
func (s *ServerError) Error() string { return "memcache [server]: " + s.Message }
