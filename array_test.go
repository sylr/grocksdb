package grocksdb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBytesToCSlice(t *testing.T) {
	t.Parallel()

	v, sizes := byteSlicesToCSlices(nil)
	require.Nil(t, v.chars)
	require.Nil(t, v.base)
	require.Nil(t, sizes)
	v.Destroy()

	keys := [][]byte{[]byte("a"), nil, []byte("bcd"), {}}
	v, sizes = byteSlicesToCSlices(keys)
	defer v.Destroy()
	require.Len(t, v.chars, len(keys))
	for i, key := range keys {
		require.EqualValues(t, len(key), sizes[i])
		require.NotNil(t, v.chars[i])
		require.Equal(t, string(key), string(refCBytes(v.chars[i], sizes[i])))
	}
}
