package signalflow

// Codec is an interface for encoding and decoding messages.
// By default, two codecs (json, binary) are available under the codec package.
type Codec interface {
	// Encode encodes the message to an array of bytes.
	Encode(m any) ([]byte, error)

	// Decode retrieves the message from an array of bytes.
	Decode(any, []byte) error

	// ContentType returns the content type of the codec.
	// Example: "application/json"
	ContentType() string
}

//go:generate mockgen -destination=./test/mocks/codec.go -package=mocks -source=codec.go
