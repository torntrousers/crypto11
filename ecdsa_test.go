// Copyright 2016, 2017 Thales e-Security, Inc
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
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	_ "crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"testing"

	"github.com/stretchr/testify/require"
)

var curves = []elliptic.Curve{
	elliptic.P224(),
	elliptic.P256(),
	elliptic.P384(),
	elliptic.P521(),
	// plus something with explicit parameters
}

func TestNativeECDSA(t *testing.T) {
	var err error
	var key *ecdsa.PrivateKey
	for _, curve := range curves {
		if key, err = ecdsa.GenerateKey(curve, rand.Reader); err != nil {
			t.Errorf("crypto.ecdsa.GenerateKey: %v", err)
			return
		}
		testEcdsaSigning(t, key, crypto.SHA1)
		testEcdsaSigning(t, key, crypto.SHA224)
		testEcdsaSigning(t, key, crypto.SHA256)
		testEcdsaSigning(t, key, crypto.SHA384)
		testEcdsaSigning(t, key, crypto.SHA512)
	}
}

func TestHardECDSA(t *testing.T) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		err = ctx.Close()
		require.NoError(t, err)
	}()

	for _, curve := range curves {
		id := randomBytes()
		label := randomBytes()

		key, err := ctx.GenerateECDSAKeyPairWithLabel(id, label, curve)
		require.NoError(t, err)
		require.NotNil(t, key)

		testEcdsaSigning(t, key, crypto.SHA1)
		testEcdsaSigning(t, key, crypto.SHA224)
		testEcdsaSigning(t, key, crypto.SHA256)
		testEcdsaSigning(t, key, crypto.SHA384)
		testEcdsaSigning(t, key, crypto.SHA512)

		key2, err := ctx.FindKeyPair(id, nil)
		require.NoError(t, err)
		testEcdsaSigning(t, key2.(*pkcs11PrivateKeyECDSA), crypto.SHA256)

		key3, err := ctx.FindKeyPair(nil, label)
		require.NoError(t, err)
		testEcdsaSigning(t, key3.(crypto.Signer), crypto.SHA384)
	}
}

func testEcdsaSigning(t *testing.T, key crypto.Signer, hashFunction crypto.Hash) {

	plaintext := []byte("sign me with ECDSA")
	h := hashFunction.New()
	_, err := h.Write(plaintext)
	require.NoError(t, err)
	plaintextHash := h.Sum([]byte{}) // weird API

	sigDER, err := key.Sign(rand.Reader, plaintextHash, nil)
	require.NoError(t, err)

	var sig dsaSignature
	err = sig.unmarshalDER(sigDER)
	require.NoError(t, err)

	ecdsaPubkey := key.Public().(crypto.PublicKey).(*ecdsa.PublicKey)
	if !ecdsa.Verify(ecdsaPubkey, plaintextHash, sig.R, sig.S) {
		t.Errorf("ECDSA Verify (hash %v): %v", hashFunction, err)
	}

}

func TestEcdsaRequiredArgs(t *testing.T) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	_, err = ctx.GenerateECDSAKeyPair(nil, elliptic.P224())
	require.Error(t, err)

	val := randomBytes()

	_, err = ctx.GenerateECDSAKeyPairWithLabel(nil, val, elliptic.P224())
	require.Error(t, err)

	_, err = ctx.GenerateECDSAKeyPairWithLabel(val, nil, elliptic.P224())
	require.Error(t, err)
}
