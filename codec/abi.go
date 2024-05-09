package codec

import (
	"bytes"
	"encoding/binary"

	signalflow "github.com/harpy-wings/signal-flow"
)

type abiCodec struct{}

var _ signalflow.Codec = (*abiCodec)(nil)

func NewABICodec() signalflow.Codec {
	return &abiCodec{}
}

func (c *abiCodec) Encode(m any) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := binary.Write(buf, binary.BigEndian, m)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (c *abiCodec) Decode(m any, bs []byte) error {
	reader := bytes.NewReader(bs)
	err := binary.Read(reader, binary.BigEndian, m)
	if err != nil {
		return err
	}
	return nil
}

func (c *abiCodec) ContentType() string {
	return "application/binary"
}
