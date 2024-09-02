package protocol

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	DataTypeRawBytes = 0x00
)

var endian = binary.BigEndian

const (
	MagicReq  = 0x80
	MagicResp = 0x81
)

type Packet struct {
	Key, Data []byte
	CAS       uint64
	Opaque    uint32
	Opcode    Opcode
	Extras    []byte
	VBucket   uint16
	scratch   []byte
}

func (p *Packet) Reset() {
	p.Key = p.Key[:0]
	p.Data = p.Data[:0]
	p.CAS = 0
	p.Opaque = 0
	p.Opcode = 0
	p.Extras = p.Extras[:0]
	p.VBucket = 0
}

/*
Field        (offset) (value)

	Magic        (0)    : 0x80
	Opcode       (1)    : 0x02
	Key length   (2,3)  : 0x0005
	Extra length (4)    : 0x08
	Data type    (5)    : 0x00
	VBucket      (6,7)  : 0x0000
	Total body   (8-11) : 0x00000012
	Opaque       (12-15): 0x00000000
	CAS          (16-23): 0x0000000000000000
	Extras              :
	Key                 :
	Value               :
*/
func (p *Packet) Write(w io.Writer) error {
	total := len(p.Extras) + len(p.Key) + len(p.Data)
	if need := 24 + total; cap(p.scratch) < need {
		p.scratch = make([]byte, 0, need)
	}
	p.scratch = p.scratch[0:24]
	{
		p.scratch[0] = MagicReq
		p.scratch[1] = byte(p.Opcode)
	}
	endian.PutUint16(p.scratch[2:4], uint16(len(p.Key)))
	{
		p.scratch[4] = byte(len(p.Extras))
		p.scratch[5] = DataTypeRawBytes
	}
	endian.PutUint16(p.scratch[6:8], p.VBucket)
	endian.PutUint32(p.scratch[8:12], uint32(total))
	endian.PutUint32(p.scratch[12:16], p.Opaque)
	endian.PutUint64(p.scratch[16:24], p.CAS)
	{
		p.scratch = append(p.scratch, p.Extras...)
		p.scratch = append(p.scratch, p.Key...)
		p.scratch = append(p.scratch, p.Data...)
	}
	if _, err := w.Write(p.scratch); err != nil {
		return err
	}
	return nil
}

/*
Field        (offset) (value)

	Magic        (0)    : 0x81
	Opcode       (1)    : 0x00
	Key length   (2,3)  : 0x0000
	Extra length (4)    : 0x04
	Data type    (5)    : 0x00
	Status       (6,7)  : 0x0000
	Total body   (8-11) : 0x00000009
	Opaque       (12-15): 0x00000000
	CAS          (16-23): 0x0000000000000001
	Extras              :
	Key                 :
	Value               :
*/
func (p *Packet) Read(r io.Reader) error {
	if need := 24; cap(p.scratch) < need {
		p.scratch = make([]byte, 0, need)
	}
	data := p.scratch[0:24]
	switch n, err := io.ReadFull(r, data); {
	case err != nil:
		return err
	case n != 24:
		return io.ErrUnexpectedEOF
	}

	if data[0] != MagicResp {
		return fmt.Errorf("memcache: bad magic number in response")
	}

	p.Opcode = Opcode(data[1])

	keyLen := int(endian.Uint16(data[2:4]))
	extras := int(data[4])

	if data[5] != DataTypeRawBytes {
		return fmt.Errorf("memcache: invalid data type")
	}

	totalLen := int(endian.Uint32(data[8:12]))

	if status := Status(endian.Uint16(data[6:8])); status != StatusOK {
		io.CopyN(io.Discard, r, int64(totalLen))
		return status
	}

	p.Opaque = endian.Uint32(data[12:16])
	p.CAS = endian.Uint64(data[16:24])

	payload := make([]byte, totalLen)
	switch n, err := io.ReadFull(r, payload); {
	case err != nil:
		return err
	case n != totalLen:
		return io.ErrUnexpectedEOF
	}
	{
		p.Extras = payload[0:extras]
		p.Key = payload[extras : extras+keyLen]
		p.Data = payload[extras+keyLen : totalLen]
	}
	return nil
}

func (p Packet) String() string {
	return fmt.Sprintf("header: \n opcode=%s, key=%s, cas=%d", p.Opcode, p.Key, p.CAS)
}
