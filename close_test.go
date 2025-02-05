// Copyright 2018 Thales e-Security, Inc
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to
// permit persons to whom the Software is furnished to do so, subject to
// the following conditions:
//
// The above copyright notice and this permission notice shall be
// included in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
// NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
// LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
// OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
// WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

package crypto11

import (
	"crypto/dsa"
	"crypto/elliptic"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClose(t *testing.T) {
	// Verify that close and re-open works.

	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	const pSize = dsa.L1024N160
	id := randomBytes()
	key, err := ctx.GenerateDSAKeyPair(id, dsaSizes[pSize])
	require.NoError(t, err)
	require.NotNil(t, key)

	require.NoError(t, ctx.Close())

	for i := 0; i < 5; i++ {
		ctx, err := ConfigureFromFile("config")
		require.NoError(t, err)

		key2, err := ctx.FindKeyPair(id, nil)
		require.NoError(t, err)

		testDsaSigning(t, key2.(*pkcs11PrivateKeyDSA), pSize, fmt.Sprintf("close%d", i))
		require.NoError(t, ctx.Close())
	}
}

// randomBytes returns 32 random bytes.
func randomBytes() []byte {
	result := make([]byte, 32)
	rand.Read(result)
	return result
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestErrorAfterClosed(t *testing.T) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	err = ctx.Close()
	require.NoError(t, err)

	bytes := randomBytes()

	_, err = ctx.FindKey(bytes, nil)
	require.Equal(t, errClosed, err)

	_, err = ctx.FindKeyPair(bytes, nil)
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateSecretKey(bytes, 256, CipherAES)
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateSecretKeyWithLabel(bytes, bytes, 256, CipherAES)
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateRSAKeyPair(bytes, 2048)
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateRSAKeyPairWithLabel(bytes, bytes, 2048)
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateDSAKeyPair(bytes, dsaSizes[dsa.L1024N160])
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateDSAKeyPairWithLabel(bytes, bytes, dsaSizes[dsa.L1024N160])
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateECDSAKeyPair(bytes, elliptic.P224())
	require.Equal(t, errClosed, err)

	_, err = ctx.GenerateECDSAKeyPairWithLabel(bytes, bytes, elliptic.P224())
	require.Equal(t, errClosed, err)

	_, err = ctx.NewRandomReader()
	require.Equal(t, errClosed, err)
}
