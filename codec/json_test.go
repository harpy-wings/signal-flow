package codec_test

import (
	"testing"

	"github.com/harpy-wings/signal-flow/codec"
	"github.com/stretchr/testify/require"
)

func TestJson(t *testing.T) {
	c := codec.NewJsonCodec()
	require.NotNil(t, c)
	t.Run("success", func(t *testing.T) {
		type testCase struct {
			Age int32
		}
		tc := testCase{Age: 12}
		bs, err := c.Encode(tc)
		require.NoError(t, err)
		require.NotEmpty(t, bs)

		var rc testCase
		err = c.Decode(&rc, bs)
		require.NoError(t, err)
		require.Equal(t, rc, tc)

		require.NotEmpty(t, c.ContentType())
	})

	t.Run("failure", func(t *testing.T) {
		type testCase struct {
			Age  int32
			Name string // since string is not a fixed size type, abi must fail.
			C    chan struct{}
		}
		tc := testCase{Age: 12, Name: "World"}
		_, err := c.Encode(tc)
		require.Error(t, err)

		var rc testCase
		err = c.Decode(&rc, []byte{})
		require.Error(t, err)
	})
}
