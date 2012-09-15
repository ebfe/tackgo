package structures

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"crypto/elliptic"
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/rand"
	"tackgo/util"	
)

const TACK_LENGTH = 166
const PUBKEY_LENGTH = 64
const HASH_LENGTH = 32
const SIG_LENGTH = 64

type Tack struct {
	PublicKey     []byte  // PUBKEY_LENGTH
	MinGeneration uint8
	Generation    uint8
	Expiration    uint32
	TargetHash    []byte  // HASH_LENGTH
	Signature     []byte  // SIG_LENGTH
}

func NewTack(publicKey []byte, minGeneration uint8, generation uint8,
	 		expiration uint32, targetHash []byte, signature []byte) (*Tack, error) {
	t := Tack{}
	t.PublicKey = publicKey[ : PUBKEY_LENGTH]
	t.MinGeneration = minGeneration
	t.Generation = generation
	t.Expiration = expiration
	t.TargetHash = targetHash[ : HASH_LENGTH]
	t.Signature = signature[ : SIG_LENGTH]
	return &t, nil
}

func NewTackFromBytes(b []byte) (*Tack, error) {
	if len(b) != TACK_LENGTH {
		return nil, fmt.Errorf("Tack is the wrong size: %d", len(b))
	}
	t := Tack{}
	t.PublicKey = b[: PUBKEY_LENGTH]
	t.MinGeneration = b[64]
	t.Generation = b[65]
	t.Expiration = uint32(b[66])<<24 | uint32(b[67])<<16 | uint32(b[68])<<8 | uint32(b[69])
	t.TargetHash = b[70 : 70 + HASH_LENGTH]
	t.Signature = b[102 : 102 + SIG_LENGTH]
	return &t, nil
}

func NewTackFromPem(s string) (*Tack, error) {
	b, err := util.Depem(s, "TACK")
	if err != nil {return nil, err}
	return NewTackFromBytes(b)
}

func (t *Tack) serializePreSig() []byte {
	buf := bytes.NewBuffer(make([]byte, 0, TACK_LENGTH))
	buf.Write(t.PublicKey)
	buf.WriteByte(t.MinGeneration)
	buf.WriteByte(t.Generation)
	binary.Write(buf, binary.BigEndian, t.Expiration)
	buf.Write(t.TargetHash)
	return buf.Bytes()
}

func (t *Tack) Serialize() []byte {
	b := t.serializePreSig()
	return append(b, t.Signature...)
}

func (t *Tack) SerializeAsPem() string {
	b := t.Serialize()
	return util.Pem(b, "TACK")
}

func (t *Tack) KeyFingerprint() string {
	return util.KeyFingerprint(t.PublicKey)
}

func (t *Tack) String() string {
	s := fmt.Sprintf(
`key fingerprint = %s
min_generation  = %d
generation      = %d
expiration      = %s
target_hash     = %s
`,
	t.KeyFingerprint(),
    t.MinGeneration,
    t.Generation,
    util.MinutesToString(t.Expiration),
    util.BytesToHexString(t.TargetHash))
	return s
}

func (t *Tack) hashForSig() []byte {
	b := t.serializePreSig()
	hash := sha256.New()
	hash.Write([]byte("tack_sig"))
	hash.Write(b)
	return hash.Sum(nil)	
}

func (t *Tack) Sign(privKey *ecdsa.PrivateKey) error {
	x, y := privKey.X, privKey.Y
	xBytes := x.Bytes()
	yBytes := y.Bytes()
	xPad := PUBKEY_LENGTH/2 - len(xBytes)
	yPad := PUBKEY_LENGTH/2 - len(yBytes)
	copy(t.PublicKey[xPad : PUBKEY_LENGTH/2], xBytes)
	copy(t.PublicKey[PUBKEY_LENGTH/2 + yPad : ], yBytes)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, t.hashForSig())
	if (err != nil) {return err}

	rBytes := r.Bytes()
	sBytes := s.Bytes()
	rPad := SIG_LENGTH/2 - len(rBytes)
	sPad := SIG_LENGTH/2 - len(sBytes)	
	copy(t.Signature[rPad : SIG_LENGTH/2], rBytes)
	copy(t.Signature[SIG_LENGTH/2 + sPad : ], sBytes)
	return nil
}

func (t *Tack) Verify() bool {
	curve := elliptic.P256()
	x, y := elliptic.Unmarshal(curve, append([]byte{4}, t.PublicKey...)) 
	pubKey := ecdsa.PublicKey{curve, x, y}
	
	var r, s big.Int
	r.SetBytes(t.Signature[ : SIG_LENGTH/2])
	s.SetBytes(t.Signature[SIG_LENGTH/2 : ])
	return ecdsa.Verify(&pubKey, t.hashForSig(), &r, &s) 
}

