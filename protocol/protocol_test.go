package protocol

import (
	"encoding/binary"
	"fmt"
	"net"
	"testing"
)

func TestSetGet(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:11211")
	if err != nil {
		t.Fatal(err)
	}
	extras := make([]byte, 8)
	binary.BigEndian.PutUint32(extras[0:4], 22) //uint32 flags
	binary.BigEndian.PutUint32(extras[4:8], 3600)
	h := Packet{
		Key:    []byte("test"),
		Data:   []byte("data"),
		Opcode: Set,
		CAS:    0,
		Extras: extras,
	}

	h.Write(conn)

	h.Read(conn)
	fmt.Println(h)

	h2 := Packet{
		Key:    []byte("test"),
		Opcode: Get,
	}
	h2.Write(conn)
	h2.Read(conn)

	fmt.Println(h2, string(h2.Data), h2.Extras)
	conn.Close()

}

func TestVersion(t *testing.T) {
	conn, err := net.Dial("tcp", "127.0.0.1:11211")
	if err != nil {
		t.Fatal(err)
	}

	h := Packet{
		Opcode: Version,
		CAS:    0,
	}

	h.Write(conn)

	h.Read(conn)
	fmt.Println(h, "version: ", string(h.Data))

	conn.Close()
}
