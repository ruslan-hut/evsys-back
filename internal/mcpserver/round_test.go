package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundFloats(t *testing.T) {
	in := `{"z":29949.1015625,"a":[1,2.5,-0.004,149.504195,1e-7,12345678901,{"t":"x 1.23456 <y>","n":null,"b":true}],"e":{},"l":[],"big":1.5e20}`
	out, err := roundFloats([]byte(in))
	require.NoError(t, err)
	// strings are re-encoded as encoding/json does everywhere: < and > escaped
	assert.Equal(t, `{"z":29949.1,"a":[1,2.5,0,149.5,0,12345678901,{"t":"x 1.23456 `+"\\u003cy\\u003e"+`","n":null,"b":true}],"e":{},"l":[],"big":150000000000000000000}`, string(out))
	assert.True(t, json.Valid(out))

	nested := `[[{"a":[{"b":1.111}]}],[]]`
	out, err = roundFloats([]byte(nested))
	require.NoError(t, err)
	assert.Equal(t, `[[{"a":[{"b":1.11}]}],[]]`, string(out), "field order and nesting kept")
}
