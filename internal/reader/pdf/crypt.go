package pdf

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rc4"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
)

// ErrPassword is returned for documents that cannot be opened without a
// password.
var ErrPassword = errors.New("this PDF is password-protected; remove the password and try again")

type cryptMethod uint8

const (
	cmNone cryptMethod = iota
	cmRC4
	cmAESV2
	cmAESV3
)

// security implements the standard security handler (RC4 40–128 bit,
// AES-128, AES-256) for documents whose user password is empty.
type security struct {
	key             []byte
	strMethod       cryptMethod
	stmMethod       cryptMethod
	filters         map[name]cryptMethod
	encryptMetadata bool
	encRef          ref
}

var passwordPad = []byte{
	0x28, 0xBF, 0x4E, 0x5E, 0x4E, 0x75, 0x8A, 0x41, 0x64, 0x00, 0x4E, 0x56, 0xFF, 0xFA, 0x01, 0x08,
	0x2E, 0x2E, 0x00, 0xB6, 0xD0, 0x68, 0x3E, 0x80, 0x2F, 0x0C, 0xA9, 0xFE, 0x64, 0x53, 0x69, 0x7A,
}

func newSecurity(ed dict, id []byte, encRef ref) (*security, error) {
	if f, _ := ed["Filter"].(name); f != "Standard" {
		return nil, fmt.Errorf("this PDF uses an unsupported encryption handler (%s)", string(f))
	}
	v, _ := integer(ed["V"])
	r, _ := integer(ed["R"])
	s := &security{encryptMetadata: true, encRef: encRef, filters: map[name]cryptMethod{}}
	if b, ok := ed["EncryptMetadata"].(bool); ok {
		s.encryptMetadata = b
	}
	o := []byte(str(ed["O"]))
	u := []byte(str(ed["U"]))
	p, _ := integer(ed["P"])

	keyLen := 5
	switch v {
	case 0, 1:
		s.strMethod, s.stmMethod = cmRC4, cmRC4
	case 2:
		s.strMethod, s.stmMethod = cmRC4, cmRC4
		keyLen = rc4KeyLen(ed["Length"])
	case 4, 5:
		cf, _ := ed["CF"].(dict)
		for fname, fv := range cf {
			fd, _ := fv.(dict)
			m := cmNone
			switch fd["CFM"] {
			case name("V2"):
				m = cmRC4
				if l := rc4KeyLen(fd["Length"]); l > keyLen {
					keyLen = l
				}
			case name("AESV2"):
				m = cmAESV2
				keyLen = 16
			case name("AESV3"):
				m = cmAESV3
				keyLen = 32
			}
			s.filters[fname] = m
		}
		s.strMethod = s.filterMethod(ed["StrF"])
		s.stmMethod = s.filterMethod(ed["StmF"])
		if v == 4 && keyLen == 5 {
			keyLen = rc4KeyLen(ed["Length"])
		}
	default:
		return nil, fmt.Errorf("this PDF uses an unsupported encryption version (V=%d)", v)
	}

	if r >= 5 {
		key, ok := aes256Key(r, o, u, []byte(str(ed["OE"])), []byte(str(ed["UE"])))
		if !ok {
			return nil, ErrPassword
		}
		s.key = key
		return s, nil
	}
	if r < 2 {
		r = 2
	}
	// Revisions 2–4 derive the key from an MD5 digest: at most 16 bytes.
	keyLen = min(max(keyLen, 5), 16)
	if key, ok := rc4Key(nil, r, keyLen, o, u, int32(p), id, s.encryptMetadata); ok {
		s.key = key
		return s, nil
	}
	// The user password may be derivable from an empty owner password.
	if upw := userFromOwner(nil, r, keyLen, o); upw != nil {
		if key, ok := rc4Key(upw, r, keyLen, o, u, int32(p), id, s.encryptMetadata); ok {
			s.key = key
			return s, nil
		}
	}
	return nil, ErrPassword
}

func str(v any) string {
	s, _ := v.(pdfString)
	return string(s)
}

// rc4KeyLen converts a /Length entry (bits, sometimes bytes) to bytes.
func rc4KeyLen(v any) int {
	n, ok := integer(v)
	if !ok || n <= 0 {
		return 5
	}
	if n <= 16 {
		return max(5, n) // given in bytes
	}
	n /= 8
	return min(max(n, 5), 16)
}

func (s *security) filterMethod(v any) cryptMethod {
	n, _ := v.(name)
	if n == "" || n == "Identity" {
		return cmNone
	}
	return s.filters[n]
}

func padPassword(pw []byte) []byte {
	out := make([]byte, 0, 32)
	out = append(out, pw[:min(len(pw), 32)]...)
	return append(out, passwordPad[:32-len(out)]...)
}

// rc4Key computes the file key from a user password (algorithm 2) and
// verifies it against /U (algorithms 4 and 5).
func rc4Key(pw []byte, r, keyLen int, o, u []byte, p int32, id []byte, encMeta bool) ([]byte, bool) {
	h := md5.New()
	h.Write(padPassword(pw))
	h.Write(o[:min(len(o), 32)])
	var pb [4]byte
	binary.LittleEndian.PutUint32(pb[:], uint32(p))
	h.Write(pb[:])
	h.Write(id)
	if r >= 4 && !encMeta {
		h.Write([]byte{0xff, 0xff, 0xff, 0xff})
	}
	sum := h.Sum(nil)
	if r >= 3 {
		for i := 0; i < 50; i++ {
			s := md5.Sum(sum[:keyLen])
			sum = s[:]
		}
	}
	key := sum[:keyLen]
	if len(u) < 16 {
		return nil, false
	}
	if r == 2 {
		got := rc4XOR(key, passwordPad)
		return key, bytes.Equal(got, u[:min(len(u), 32)])
	}
	h = md5.New()
	h.Write(passwordPad)
	h.Write(id)
	x := rc4XOR(key, h.Sum(nil))
	tmp := make([]byte, len(key))
	for i := 1; i <= 19; i++ {
		for j := range key {
			tmp[j] = key[j] ^ byte(i)
		}
		x = rc4XOR(tmp, x)
	}
	return key, bytes.Equal(x[:16], u[:16])
}

// userFromOwner recovers the padded user password from /O using an owner
// password (algorithm 7).
func userFromOwner(ownerPW []byte, r, keyLen int, o []byte) []byte {
	if len(o) < 32 {
		return nil
	}
	sum := md5.Sum(padPassword(ownerPW))
	k := sum[:]
	if r >= 3 {
		for i := 0; i < 50; i++ {
			s := md5.Sum(k)
			k = s[:]
		}
	}
	key := k[:keyLen]
	if r == 2 {
		return rc4XOR(key, o[:32])
	}
	x := append([]byte(nil), o[:32]...)
	tmp := make([]byte, len(key))
	for i := 19; i >= 0; i-- {
		for j := range key {
			tmp[j] = key[j] ^ byte(i)
		}
		x = rc4XOR(tmp, x)
	}
	return x
}

func rc4XOR(key, data []byte) []byte {
	c, err := rc4.NewCipher(key)
	if err != nil {
		return nil
	}
	out := make([]byte, len(data))
	c.XORKeyStream(out, data)
	return out
}

// aes256Key validates the empty password as user or owner password of an
// AES-256 document (revisions 5 and 6) and returns the file key.
func aes256Key(r int, o, u, oe, ue []byte) ([]byte, bool) {
	if len(u) < 48 || len(o) < 48 || len(ue) < 32 || len(oe) < 32 {
		return nil, false
	}
	if bytes.Equal(hashR6(r, nil, u[32:40], nil), u[:32]) {
		return aesNoIV(hashR6(r, nil, u[40:48], nil), ue[:32])
	}
	if bytes.Equal(hashR6(r, nil, o[32:40], u[:48]), o[:32]) {
		return aesNoIV(hashR6(r, nil, o[40:48], u[:48]), oe[:32])
	}
	return nil, false
}

func aesNoIV(key, data []byte) ([]byte, bool) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, false
	}
	out := make([]byte, len(data))
	cipher.NewCBCDecrypter(block, make([]byte, aes.BlockSize)).CryptBlocks(out, data)
	return out, true
}

// hashR6 is the password hash of revision 5 (one SHA-256) and revision 6
// (algorithm 2.B, iterated SHA-256/384/512 with AES-128).
func hashR6(r int, pw, salt, udata []byte) []byte {
	h := sha256.New()
	h.Write(pw)
	h.Write(salt)
	h.Write(udata)
	k := h.Sum(nil)
	if r < 6 {
		return k
	}
	for i := 0; ; i++ {
		seq := make([]byte, 0, len(pw)+len(k)+len(udata))
		seq = append(append(append(seq, pw...), k...), udata...)
		k1 := bytes.Repeat(seq, 64)
		block, err := aes.NewCipher(k[:16])
		if err != nil {
			return nil
		}
		e := make([]byte, len(k1))
		cipher.NewCBCEncrypter(block, k[16:32]).CryptBlocks(e, k1)
		sum := 0
		for _, c := range e[:16] {
			sum += int(c)
		}
		var hh hash.Hash
		switch sum % 3 {
		case 0:
			hh = sha256.New()
		case 1:
			hh = sha512.New384()
		default:
			hh = sha512.New()
		}
		hh.Write(e)
		k = hh.Sum(nil)
		if i >= 63 && int(e[len(e)-1]) <= i+1-32 {
			break
		}
		if i > 1000 {
			break
		}
	}
	return k[:32]
}

func (s *security) isEncryptDict(r ref) bool { return s.encRef.num > 0 && r == s.encRef }

func (s *security) objectKey(r ref, m cryptMethod) []byte {
	if m == cmAESV3 {
		return s.key
	}
	h := md5.New()
	h.Write(s.key)
	h.Write([]byte{byte(r.num), byte(r.num >> 8), byte(r.num >> 16), byte(r.gen), byte(r.gen >> 8)})
	if m == cmAESV2 {
		h.Write([]byte("sAlT"))
	}
	return h.Sum(nil)[:min(len(s.key)+5, 16)]
}

func (s *security) decrypt(data []byte, r ref, m cryptMethod) []byte {
	switch m {
	case cmRC4:
		return rc4XOR(s.objectKey(r, m), data)
	case cmAESV2, cmAESV3:
		return aesCBCDecrypt(s.objectKey(r, m), data)
	}
	return data
}

func aesCBCDecrypt(key, data []byte) []byte {
	if len(data) < 2*aes.BlockSize {
		return nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil
	}
	iv, body := data[:aes.BlockSize], data[aes.BlockSize:]
	body = body[:len(body)/aes.BlockSize*aes.BlockSize]
	out := make([]byte, len(body))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(out, body)
	if n := len(out); n > 0 {
		pad := int(out[n-1])
		if pad >= 1 && pad <= aes.BlockSize && pad <= n {
			valid := true
			for _, c := range out[n-pad:] {
				if int(c) != pad {
					valid = false
					break
				}
			}
			if valid {
				out = out[:n-pad]
			}
		}
	}
	return out
}

// decryptObject decrypts every string inside obj (stream data is decrypted
// lazily when decoded).
func (s *security) decryptObject(obj any, r ref) any {
	switch x := obj.(type) {
	case pdfString:
		return pdfString(s.decrypt([]byte(x), r, s.strMethod))
	case array:
		for i, v := range x {
			x[i] = s.decryptObject(v, r)
		}
		return x
	case dict:
		for k, v := range x {
			x[k] = s.decryptObject(v, r)
		}
		return x
	case *stream:
		s.decryptObject(x.d, r)
		return x
	}
	return obj
}

// streamMethod selects the cipher for a stream, honouring /Crypt filters
// and unencrypted metadata.
func (s *security) streamMethod(st *stream) cryptMethod {
	if st.d["Type"] == name("Metadata") && !s.encryptMetadata {
		return cmNone
	}
	filters := filterList(st.d["Filter"])
	if len(filters) > 0 && filters[0] == "Crypt" {
		parms := parmsAt(st.d["DecodeParms"], 0)
		n, _ := parms["Name"].(name)
		if n == "" || n == "Identity" {
			return cmNone
		}
		return s.filters[n]
	}
	return s.stmMethod
}
