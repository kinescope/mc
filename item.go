package mc

import (
	"encoding"
	"encoding/json"

	"github.com/tinylib/msgp/msgp"
	"google.golang.org/protobuf/proto"
)

type (
	Item struct {
		Key   string
		Value Value
		Flags uint16
		cas   uint64
		win   bool
	}
	Value []byte
)

func (i *Item) Win() bool { return i.win }

func (val *Value) Marshal(v any) (err error) {
	r, err := marshal(v)
	if err != nil {
		return err
	}
	*val = r
	return nil
}

func (val Value) Unmarshal(v any) error {
	return unmarshal(val, v)
}

func marshal(v any) ([]byte, error) {
	switch v := v.(type) {
	case proto.Message:
		return proto.Marshal(v)
	case msgp.Marshaler:
		return v.MarshalMsg(nil)
	case encoding.BinaryMarshaler:
		return v.MarshalBinary()
	}
	return json.Marshal(v)
}

func unmarshal(data []byte, v any) error {
	switch v := v.(type) {
	case proto.Message:
		return proto.Unmarshal(data, v)
	case msgp.Unmarshaler:
		if _, err := v.UnmarshalMsg(data); err != nil {
			return err
		}
		return nil
	case encoding.BinaryUnmarshaler:
		return v.UnmarshalBinary(data)
	}
	return json.Unmarshal(data, v)
}
