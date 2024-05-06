package codec

import (
	"encoding/json"

	signalflow "github.com/harpy-wings/signal-flow"
)

type jsonCodec struct{}

var _ signalflow.Codec = (*jsonCodec)(nil)

func NewJsonCodec() signalflow.Codec {
	return &jsonCodec{}
}

func (c *jsonCodec) Encode(m any) ([]byte, error) {
	return json.Marshal(m)
}

func (c *jsonCodec) Decode(m any, bs []byte) error {
	return json.Unmarshal(bs, m)
}

func (c *jsonCodec) ContentType() string {
	return "application/json"
}
