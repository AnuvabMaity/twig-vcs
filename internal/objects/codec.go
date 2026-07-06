package objects

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// encMode is the cached canonical CBOR encoding mode.
var encMode cbor.EncMode

// decMode is the cached CBOR decoding mode.
var decMode cbor.DecMode

func init() {
	var err error
	encMode, err = cbor.CanonicalEncOptions().EncMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create canonical CBOR encoder: %v", err))
	}

	decMode, err = cbor.DecOptions{}.DecMode()
	if err != nil {
		panic(fmt.Sprintf("failed to create CBOR decoder: %v", err))
	}
}

// Encode serializes the value v into canonical (deterministic) CBOR bytes.
func Encode(v interface{}) ([]byte, error) {
	return encMode.Marshal(v)
}

// Decode deserializes canonical CBOR bytes data into the pointer v.
func Decode(data []byte, v interface{}) error {
	return decMode.Unmarshal(data, v)
}
