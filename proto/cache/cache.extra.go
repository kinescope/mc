package cache

import "google.golang.org/protobuf/proto"

func (i *Item) Marshal() ([]byte, error) { return proto.Marshal(i) }
func (i *Item) Unmarshal(b []byte) error { return proto.Unmarshal(b, i) }
