package mc_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/bytedance/sonic"
	"github.com/kinescope/mc"
	"github.com/kinescope/mc/proto/cache"
	"github.com/stretchr/testify/assert"
)

func TestItemSerializeProtobuf(t *testing.T) {
	var (
		i mc.Item
		v = cache.Item{
			Data: []byte{1, 2, 3, 4, 5},
			Namespace: &cache.Namespace{
				Key: "k",
				Ver: 12345,
			},
		}
	)
	if err := i.Value.Marshal(&v); assert.NoError(t, err) {
		var v2 cache.Item
		if err := i.Value.Unmarshal(&v2); assert.NoError(t, err) {
			assert.Equal(t, v.Data, v2.Data)
			assert.Equal(t, v.Namespace.Key, v2.Namespace.Key)
			assert.Equal(t, v.Namespace.Ver, v2.Namespace.Ver)
		}
	}
}

//go:generate msgp
type MsgPackTest struct {
	Data string
}

func TestItemSerializeMessagePack(t *testing.T) {
	var (
		i mc.Item
		v = MsgPackTest{
			Data: randSeq(42),
		}
	)
	if err := i.Value.Marshal(&v); assert.NoError(t, err) {
		var v2 MsgPackTest
		if err := i.Value.Unmarshal(&v2); assert.NoError(t, err) {
			assert.Equal(t, v.Data, v2.Data)
		}
	}
}

type BinTest struct {
	Data []byte
}

func (b *BinTest) MarshalBinary() (data []byte, err error) { return b.Data, nil }
func (b *BinTest) UnmarshalBinary(data []byte) error {
	b.Data = data
	return nil
}

func TestItemSerializeBin(t *testing.T) {
	var (
		i mc.Item
		v = BinTest{
			Data: []byte(randSeq(42)),
		}
	)
	if err := i.Value.Marshal(&v); assert.NoError(t, err) {
		var v2 BinTest
		if err := i.Value.Unmarshal(&v2); assert.NoError(t, err) {
			assert.Equal(t, v.Data, v2.Data)
		}
	}
}

type JsonTest struct {
	Data string
}

func TestItemSerializeJson(t *testing.T) {
	var (
		i mc.Item
		v = JsonTest{
			Data: randSeq(42),
		}
	)
	if err := i.Value.Marshal(&v); assert.NoError(t, err) && json.Valid(i.Value) {
		var v2 JsonTest
		if err := i.Value.Unmarshal(&v2); assert.NoError(t, err) {
			assert.Equal(t, v.Data, v2.Data)
		}
	}
}

type BenchStruct struct {
	A string
	B int
	C time.Time
}

func BenchmarkSonic(b *testing.B) {
	t := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		data, err := sonic.Marshal(BenchStruct{
			A: "test",
			B: 24,
			C: t,
		})
		if err != nil {
			b.Fatal(err)
		}
		var v BenchStruct
		if err := sonic.Unmarshal(data, &v); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkJson(b *testing.B) {
	t := time.Now()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		data, err := json.Marshal(BenchStruct{
			A: "test",
			B: 24,
			C: t,
		})
		if err != nil {
			b.Fatal(err)
		}
		var v BenchStruct
		if err := json.Unmarshal(data, &v); err != nil {
			b.Fatal(err)
		}
	}
}
