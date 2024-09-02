package mc

import (
	"encoding"
	"encoding/json"

	"google.golang.org/protobuf/proto"
)

type (
	Item struct {
		Key   string
		Value Value
		Flags uint16
		cas   uint64
	}
	Value []byte
)

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
	case encoding.BinaryMarshaler:
		return v.MarshalBinary()
	}
	return json.Marshal(v)
}

func unmarshal(data []byte, v any) error {
	switch v := v.(type) {
	case proto.Message:
		return proto.Unmarshal(data, v)
	case encoding.BinaryUnmarshaler:
		return v.UnmarshalBinary(data)
	}
	return json.Unmarshal(data, v)
}
