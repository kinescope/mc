package protocol

import "fmt"

type Opcode byte

// https://github.com/memcached/memcached/wiki/BinaryProtocolRevamped#command-opcodes
const (
	Get       Opcode = 0x00
	Set              = 0x01
	Add              = 0x02
	Replace          = 0x03
	Delete           = 0x04
	Increment        = 0x05
	Decrement        = 0x06
	Noop             = 0x0a
	Version          = 0x0b
	GetKQ            = 0x0d
)

func (c Opcode) String() string {
	switch c {
	case Get:
		return "get"
	case Set:
		return "set"
	case Version:
		return "version"
	}
	return fmt.Sprintf("undefined Opcode: %d", c)
}
