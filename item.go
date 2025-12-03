package mc

import (
	"encoding"
	"encoding/json"

	"github.com/bytedance/sonic"
	"github.com/tinylib/msgp/msgp"
	"google.golang.org/protobuf/proto"
)

type (
	Item struct {
		Key        string
		Value      Value
		Flags      uint16
		cr         float32 // compression ratio
		cas        uint64
		hit        bool
		won        bool
		stale      bool
		opaque     int
		lastAccess int64
	}
	Value []byte
)

// Won returns true if this client "won" the early recache flag.
// When multiple clients request an item with WithEarlyRecache(), only one
// client wins and should refresh the cache in the background.
func (i *Item) Won() bool { return i.won }

// Hit returns true if the item has been accessed before.
// This is useful for identifying hot cache items that are frequently accessed.
func (i *Item) Hit() bool { return i.hit }

// Stale returns true if the item has expired but is still being served.
// Stale items are served to prevent cache misses during refresh operations.
func (i *Item) Stale() bool { return i.stale }

// LastAccess returns the time in seconds since the item was last accessed.
// This is useful for cache optimization and identifying frequently used items.
func (i *Item) LastAccess() int { return int(i.lastAccess) }

// CompressionRatio returns the compression ratio if the item was compressed.
// A value greater than 1.0 indicates the data was compressed.
// Returns 0.0 if the item was not compressed.
func (i *Item) CompressionRatio() float32 { return i.cr }

// Marshal serializes the given value into the Value using the appropriate
// encoding based on the value's type. Supports protobuf, JSON, MessagePack,
// and binary marshaling interfaces, falling back to JSON encoding.
func (val *Value) Marshal(v any) error {
	r, err := marshal(v)
	if err != nil {
		return err
	}
	*val = r
	return nil
}

// Unmarshal deserializes the Value into the given value using the appropriate
// decoding based on the value's type. Supports protobuf, JSON, MessagePack,
// and binary unmarshaling interfaces, falling back to JSON decoding.
func (val Value) Unmarshal(v any) error {
	return unmarshal(val, v)
}

func marshal(v any) ([]byte, error) {
	switch v := v.(type) {
	case proto.Message:
		return proto.Marshal(v)
	case json.Marshaler:
		return v.MarshalJSON()
	case msgp.Marshaler:
		return v.MarshalMsg(nil)
	case encoding.BinaryMarshaler:
		return v.MarshalBinary()
	}
	return sonic.Marshal(v)
}

func unmarshal(data []byte, v any) error {
	switch v := v.(type) {
	case proto.Message:
		return proto.Unmarshal(data, v)
	case json.Unmarshaler:
		return v.UnmarshalJSON(data)
	case msgp.Unmarshaler:
		if _, err := v.UnmarshalMsg(data); err != nil {
			return err
		}
		return nil
	case encoding.BinaryUnmarshaler:
		return v.UnmarshalBinary(data)
	}
	return sonic.Unmarshal(data, v)
}
