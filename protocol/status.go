package protocol

import "fmt"

// https://github.com/memcached/memcached/wiki/BinaryProtocolRevamped#response-status
type Status uint16

const (
	StatusOK                               Status = 0x0000
	StatusKeyNotFound                             = 0x0001
	StatusKeyExists                               = 0x0002
	StatusValueTooLarge                           = 0x0003
	StatusInvalidArguments                        = 0x0004
	StatusItemNotStored                           = 0x0005
	StatusIncrDecrOnNonNumericValue               = 0x0006
	StatusTheVBucketBelongsToAnotherServer        = 0x0007
	StatusAuthenticationError                     = 0x0008
	StatusAuthenticationContinue                  = 0x0009
	StatusUnknownCommand                          = 0x0081
	StatusOutOfMemory                             = 0x0082
	StatusNotSupported                            = 0x0083
	StatusInternalError                           = 0x0084
	StatusBusy                                    = 0x0085
	StatusTemporaryFailure                        = 0x0086
)

func (s Status) Error() string {
	return fmt.Sprintf("memcache: status=%d", s)
}
