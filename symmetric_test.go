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
	"bytes"
	"crypto/cipher"
	"runtime"
	"testing"

	"github.com/miekg/pkcs11"
	"github.com/stretchr/testify/require"
)

func TestHardSymmetric(t *testing.T) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	t.Run("AES128", func(t *testing.T) { testHardSymmetric(t, ctx, pkcs11.CKK_AES, 128) })
	t.Run("AES192", func(t *testing.T) { testHardSymmetric(t, ctx, pkcs11.CKK_AES, 192) })
	t.Run("AES256", func(t *testing.T) { testHardSymmetric(t, ctx, pkcs11.CKK_AES, 256) })
	t.Run("DES3", func(t *testing.T) { testHardSymmetric(t, ctx, pkcs11.CKK_DES3, 0) })
}

func testHardSymmetric(t *testing.T, ctx *Context, keytype int, bits int) {

	id := randomBytes()
	key, err := ctx.GenerateSecretKey(id, bits, Ciphers[keytype])
	require.NoError(t, err)
	require.NotNil(t, key)

	var key2 *SecretKey
	t.Run("Find", func(t *testing.T) {
		key2, err = ctx.FindKey(id, nil)
		require.NoError(t, err)
	})

	t.Run("Block", func(t *testing.T) { testSymmetricBlock(t, key, key2) })

	iv := make([]byte, key.BlockSize())
	for i := range iv {
		iv[i] = 0xF0
	}

	t.Run("CBC", func(t *testing.T) {
		testSymmetricMode(t, cipher.NewCBCEncrypter(key2, iv), cipher.NewCBCDecrypter(key2, iv))
	})

	t.Run("CBCClose", func(t *testing.T) {

		enc, err := key2.NewCBCEncrypterCloser(iv)
		require.NoError(t, err)

		dec, err := key2.NewCBCDecrypterCloser(iv)
		require.NoError(t, err)

		testSymmetricMode(t, enc, dec)
		enc.Close()
		dec.Close()
	})

	t.Run("CBCNoClose", func(t *testing.T) {
		enc, err := key2.NewCBCEncrypter(iv)
		require.NoError(t, err)

		dec, err := key2.NewCBCDecrypter(iv)
		require.NoError(t, err)
		testSymmetricMode(t, enc, dec)

		// See discussion at BlockModeCloser.
		runtime.GC()
	})

	t.Run("CBCSealOpen", func(t *testing.T) {
		aead, err := key2.NewCBC(PaddingNone)
		require.NoError(t, err)
		testAEADMode(t, aead, 128, 0)
	})

	t.Run("CBCPKCSSealOpen", func(t *testing.T) {
		aead, err := key2.NewCBC(PaddingPKCS)
		require.NoError(t, err)
		testAEADMode(t, aead, 127, 0)
	})
	if bits == 128 {
		t.Run("GCMSoft", func(t *testing.T) {
			aead, err := cipher.NewGCM(key2)
			require.NoError(t, err)
			testAEADMode(t, aead, 127, 129)
		})
		t.Run("GCMHard", func(t *testing.T) {
			aead, err := key2.NewGCM()
			require.NoError(t, err)
			skipIfMechUnsupported(t, key2.context, pkcs11.CKM_AES_GCM)
			testAEADMode(t, aead, 127, 129)
		})
		// TODO check that hard/soft is consistent!
	}
	// TODO CFB
	// TODO OFB
	// TODO CTR

}

func testSymmetricBlock(t *testing.T, encryptKey cipher.Block, decryptKey cipher.Block) {
	b := encryptKey.BlockSize()
	input := make([]byte, 3*b)
	middle := make([]byte, 3*b)
	output := make([]byte, 3*b)
	// Set a recognizable pattern in the buffers
	for i := 0; i < 3*b; i++ {
		input[i] = byte(i)
		middle[i] = byte(i + 3*b)
		output[i] = byte(i + 6*b)
	}
	encryptKey.Encrypt(middle, input) // middle[:b] = encrypt(input[:b])
	if bytes.Equal(input[:b], middle[:b]) {
		t.Errorf("crypto11.PKCSSecretKey.Encrypt: identity transformation")
		return
	}
	matches := 0
	for i := 0; i < b; i++ {
		if middle[i] == byte(i+3*b) {
			matches++
		}
	}
	if matches == b {
		t.Errorf("crypto11.PKCSSecretKey.Encrypt: didn't modify destination")
		return
	}
	for i := 0; i < 3*b; i++ {
		if input[i] != byte(i) {
			t.Errorf("crypto11.PKCSSecretKey.Encrypt: corrupted source")
			return
		}
		if i >= b && middle[i] != byte(i+3*b) {
			t.Errorf("crypto11.PKCSSecretKey.Encrypt: corrupted destination past blocksize")
			return
		}
	}
	decryptKey.Decrypt(output, middle) // output[:b] = decrypt(middle[:b])
	if !bytes.Equal(input[:b], output[:b]) {
		t.Errorf("crypto11.PKCSSecretKey.Decrypt: plaintext wrong")
		return
	}
	for i := 0; i < 3*b; i++ {
		if i >= b && output[i] != byte(i+6*b) {
			t.Errorf("crypto11.PKCSSecretKey.Decrypt: corrupted destination past blocksize")
			return
		}
	}
}

func testSymmetricMode(t *testing.T, encrypt cipher.BlockMode, decrypt cipher.BlockMode) {
	input := make([]byte, 256)
	middle := make([]byte, 256)
	output := make([]byte, 256)
	// Set a recognizable pattern in the buffers
	for i := 0; i < 256; i++ {
		input[i] = byte(i)
		middle[i] = byte(i + 32)
		output[i] = byte(i + 64)
	}
	// Encrypt the first 128 bytes
	encrypt.CryptBlocks(middle, input[:128])
	if bytes.Equal(input[:128], middle[:128]) {
		t.Errorf("BlockMode.Encrypt: did not modify destination")
		return
	}
	for i := 0; i < 128; i++ {
		if input[i] != byte(i) {
			t.Errorf("BlockMode.Encrypt: corrupted source")
			return
		}
	}
	for i := 128; i < 256; i++ {
		if middle[i] != byte(i+32) {
			t.Errorf("BlockMode.Encrypt: corrupted destination past input size")
			return
		}
	}
	// Encrypt the rest
	encrypt.CryptBlocks(middle[128:], input[128:])
	// Decrypt in a single go
	decrypt.CryptBlocks(output, middle)
	if !bytes.Equal(input, output) {
		t.Errorf("BlockMode.Decrypt: plaintext wrong")
		return
	}
}

func testAEADMode(t *testing.T, aead cipher.AEAD, ptlen int, adlen int) {
	nonce := make([]byte, aead.NonceSize())
	plaintext := make([]byte, ptlen)
	for i := 0; i < len(plaintext); i++ {
		plaintext[i] = byte(i)
	}
	additionalData := make([]byte, adlen)
	for i := 0; i < len(additionalData); i++ {
		additionalData[i] = byte(i + 16)
	}
	ciphertext := aead.Seal([]byte{}, nonce, plaintext, additionalData)
	decrypted, err := aead.Open([]byte{}, nonce, ciphertext, additionalData)
	if err != nil {
		t.Errorf("aead.Open: %s", err)
		return
	}
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("aead.Open: mismatch")
		return
	}
}

func BenchmarkCBC(b *testing.B) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(b, err)

	defer func() {
		require.NoError(b, ctx.Close())
	}()

	id := randomBytes()
	key, err := ctx.GenerateSecretKey(id, 128, Ciphers[pkcs11.CKK_AES])
	require.NoError(b, err)

	iv := make([]byte, 16)
	plaintext := make([]byte, 65536)
	ciphertext := make([]byte, 65536)

	b.Run("Native", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mode := cipher.NewCBCEncrypter(key, iv)
			mode.CryptBlocks(ciphertext, plaintext)
		}
	})

	b.Run("IdiomaticClose", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mode, err := key.NewCBCEncrypterCloser(iv)
			if err != nil {
				panic(err)
			}
			mode.CryptBlocks(ciphertext, plaintext)
			mode.Close()
		}
	})

	b.Run("Idiomatic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			mode, err := key.NewCBCEncrypter(iv)
			if err != nil {
				panic(err)
			}
			mode.CryptBlocks(ciphertext, plaintext)
		}
		runtime.GC()
	})
}

func TestSymmetricRequiredArgs(t *testing.T) {
	ctx, err := ConfigureFromFile("config")
	require.NoError(t, err)

	defer func() {
		require.NoError(t, ctx.Close())
	}()

	_, err = ctx.GenerateSecretKey(nil, 128, CipherAES)
	require.Error(t, err)

	val := randomBytes()

	_, err = ctx.GenerateSecretKeyWithLabel(nil, val, 128, CipherAES)
	require.Error(t, err)

	_, err = ctx.GenerateSecretKeyWithLabel(val, nil, 128, CipherAES)
	require.Error(t, err)
}

// TODO BenchmarkGCM along the same lines as above
