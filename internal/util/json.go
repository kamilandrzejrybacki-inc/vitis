package util

import (
	"encoding/base64"
	"encoding/json"
)

type Bytes []byte

func (b Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(base64.StdEncoding.EncodeToString([]byte(b)))
}
