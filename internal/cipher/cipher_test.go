package cipher_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

// https://github.com/starius/aesctrat?tab=readme-ov-file
func Test_part(t *testing.T) {
	// Load your secret key from a safe place and reuse it across multiple
	// NewCipher calls. (Obviously don't use this example key for anything
	// real.) If you want to convert a passphrase to a key, use a suitable
	// package like bcrypt or scrypt.
	key, _ := hex.DecodeString("6368616e676520746869732070617373")

	plaintext := bytes.NewReader([]byte("some secret text\n"))

	wblock, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// If the key is unique for each ciphertext, then it's ok to use a zero
	// IV.
	var wiv [aes.BlockSize]byte
	wstream := cipher.NewOFB(wblock, wiv[:])

	var out bytes.Buffer

	writer := &cipher.StreamWriter{S: wstream, W: &out}
	// Copy the input to the output buffer, encrypting as we go.
	if _, err := io.Copy(writer, plaintext); err != nil {
		panic(err)
	}

	fmt.Printf("%x\n", out.Bytes())

	encrypted := bytes.NewReader(out.Bytes())

	rblock, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	var riv [aes.BlockSize]byte
	rstream := cipher.NewOFB(rblock, riv[:])

	reader := &cipher.StreamReader{S: rstream, R: encrypted}
	// Copy the input to the output stream, decrypting as we go.
	if _, err := io.Copy(os.Stdout, reader); err != nil {
		panic(err)
	}
}
