package protocol

import "fmt"

type Opcode byte

// https://github.com/memcached/memcached/wiki/BinaryProtocolRevamped#command-opcodes
const (
	Get       Opcode = 0x00
	Set              = 0x01
	Add              = 0x02
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
	case Add:
		return "add"
	case Noop:
		return "noop"
	case GetKQ:
		return "get_kq"
	case Delete:
		return "delete"
	case Version:
		return "version"
	case Increment:
		return "increment"
	case Decrement:
		return "decrement"
	}
	return fmt.Sprintf("undefined Opcode: %d", c)
}
