package mc_test

import (
	"encoding/json"
	"testing"

	"github.com/kinescope/mc"
	"github.com/kinescope/mc/proto/cache"
	"github.com/stretchr/testify/assert"
)

func TestValueSerialize(t *testing.T) {
	{
		var (
			value mc.Value
			data  = Bin{
				F1: randSeq(2),
				F2: randSeq(8),
				F3: 1,
				F4: 2,
			}
		)
		if err := value.Marshal(&data); assert.NoError(t, err) {
			var d1 Bin
			if err := value.Unmarshal(&d1); assert.NoError(t, err) {
				assert.Equal(t, data, d1)
			}
		}
	}
	{
		var (
			value mc.Value
			data  = cache.Item{
				Data: []byte(randSeq(42)),
				Namespace: &cache.Namespace{
					Key: randSeq(2),
				},
			}
		)
		if err := value.Marshal(&data); assert.NoError(t, err) {
			if assert.False(t, json.Valid(value)) {
				var d1 cache.Item
				if err := value.Unmarshal(&d1); assert.NoError(t, err) {
					assert.Equal(t, data.Data, d1.Data)
					assert.Equal(t, data.Namespace.Key, d1.Namespace.Key)
				}
			}
		}
	}
	{
		type Example struct {
			A string
			B string
		}
		var (
			value mc.Value
			data  = Example{
				A: randSeq(6),
				B: randSeq(8),
			}
		)
		if err := value.Marshal(&data); assert.NoError(t, err) {
			if assert.True(t, json.Valid(value)) {
				var d1 Example
				if err := value.Unmarshal(&d1); assert.NoError(t, err) {
					assert.Equal(t, data, d1)
				}
			}
		}
	}
}

type Bin struct {
	F1, F2 string
	F3, F4 int
}

func (b *Bin) MarshalBinary() ([]byte, error) {
	return json.Marshal(b)
}

func (b *Bin) UnmarshalBinary(d []byte) error {
	return json.Unmarshal(d, b)
}
