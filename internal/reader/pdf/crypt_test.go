package pdf

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/askrejans/crowdoc/v2/internal/reader/rd"
)

func TestAESCBC(t *testing.T) {
	key := bytes.Repeat([]byte{7}, 16)
	iv := bytes.Repeat([]byte{9}, 16)
	plain := []byte("seventeen bytes!!")
	pad := 16 - len(plain)%16
	padded := append(append([]byte(nil), plain...), bytes.Repeat([]byte{byte(pad)}, pad)...)
	block, _ := aes.NewCipher(key)
	enc := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(enc, padded)
	if got := aesCBCDecrypt(key, append(append([]byte(nil), iv...), enc...)); !bytes.Equal(got, plain) {
		t.Errorf("got %q", got)
	}
	// Too short for an IV and a block: nothing, and no panic.
	if got := aesCBCDecrypt(key, iv); got != nil {
		t.Errorf("short data: %q", got)
	}
	// A ragged tail is ignored rather than crashing the block cipher.
	if got := aesCBCDecrypt(key, append(append(append([]byte(nil), iv...), enc...), 1, 2, 3)); !bytes.Equal(got, plain) {
		t.Errorf("ragged: %q", got)
	}
}

func TestSecurityDictionary(t *testing.T) {
	if _, err := newSecurity(dict{"Filter": name("PubSec")}, nil, ref{}); err == nil || !strings.Contains(err.Error(), "PubSec") {
		t.Errorf("public-key handler: %v", err)
	}
	if _, err := newSecurity(dict{"Filter": name("Standard"), "V": 9}, nil, ref{}); err == nil {
		t.Error("unknown version accepted")
	}
	// Garbage /O and /U values cannot be validated: the file counts as
	// password-protected rather than being decrypted with a wrong key.
	_, err := newSecurity(dict{"Filter": name("Standard"), "V": 2, "R": 3, "Length": 128,
		"O": pdfString(bytes.Repeat([]byte{1}, 32)), "U": pdfString(bytes.Repeat([]byte{2}, 32)), "P": -4}, []byte("id"), ref{})
	if !errors.Is(err, ErrPassword) {
		t.Errorf("bad /U: %v", err)
	}
	for in, want := range map[int]int{40: 5, 128: 16, 16: 16, 5: 5, 1024: 16, 0: 5} {
		if got := rc4KeyLen(in); got != want {
			t.Errorf("rc4KeyLen(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestPasswordProtected(t *testing.T) {
	for _, name := range []string{"password.pdf", "password-rc4.pdf"} {
		data, err := os.ReadFile("testdata/" + name)
		if err != nil {
			t.Fatal(err)
		}
		_, _, err = Read(context.Background(), data, rd.Options{})
		if !errors.Is(err, ErrPassword) || err.Error() != "this PDF is password-protected; remove the password and try again" {
			t.Errorf("%s: %v", name, err)
		}
	}
}
