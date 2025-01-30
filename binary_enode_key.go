package mc

import (
	"sync"

	"github.com/cespare/xxhash/v2"
	"github.com/cristalhq/base64"
)

var xxHashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

var base64Enc = base64.StdEncoding

var (
	binaryEncodeKey = func(key string) (string, error) {
		hash := xxHashPool.Get().(*xxhash.Digest)
		hash.Reset()
		hash.WriteString(key)
		sum := hash.Sum(nil)
		{
			xxHashPool.Put(hash)
		}
		return base64Enc.EncodeToString(sum), nil
	}
)
