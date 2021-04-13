// Copyright (c) 2020-2021 Sippy Software, Inc. All rights reserved.
//
// All rights reserved.
//
// Redistribution and use in source and binary forms, with or without modification,
// are permitted provided that the following conditions are met:
//
// 1. Redistributions of source code must retain the above copyright notice, this
// list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright notice,
// this list of conditions and the following disclaimer in the documentation and/or
// other materials provided with the distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
// ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR
// ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
// LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
// ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
// SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
package sippy_security

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "encoding/binary"
    "encoding/hex"

    "sippy/time"
    "sippy/utils"
)

const (
    VTIME = int64(32000000000)
)

type hashOracle struct {
    ac      *AESCipher
}

type AESCipher struct {
    cipher  cipher.Block
}

var _key []byte
var HashOracle *hashOracle

func init() {
    var err error

    _key = make([]byte, aes.BlockSize * 2)
    rand.Read(_key)
    HashOracle, err = newHashOracle()
    if err != nil {
        panic("hashOracle cannot be created: " + err.Error())
    }
}

func newHashOracle() (*hashOracle, error) {
    ac, err := NewAESCipher()
    if err != nil {
        return nil, err
    }
    return &hashOracle{
        ac  : ac,
    }, nil
}

func (self *hashOracle) EmitChallenge(cmask int64) string {
    now, _ := sippy_time.ClockGettime(sippy_time.CLOCK_MONOTONIC)
    ts64 := (now.UnixNano() << NUM_OF_DGSTS) | cmask
    return self.ac.Encrypt(ts64)
}

func (self *hashOracle) ValidateChallenge(cryptic string, cmask int64) bool {
    now, _ := sippy_time.ClockGettime(sippy_time.CLOCK_MONOTONIC)
    new_ts := now.UnixNano()
    decryptic, err := self.ac.Decrypt(cryptic)
    if err != nil || (cmask & decryptic) == 0 {
        return false
    }
    orig_ts := decryptic >> NUM_OF_DGSTS
    tsdiff := new_ts - orig_ts
    if tsdiff < 0 || tsdiff > VTIME {
        return false
    }
    return true
}

func NewAESCipher() (*AESCipher, error) {
    cipher, err := aes.NewCipher(_key)
    if err != nil {
        return nil, err
    }
    return &AESCipher{
        cipher  : cipher,
    }, nil
}

func (self *AESCipher) Encrypt(ts64 int64) string {
    buf := make([]byte, 8)
    binary.BigEndian.PutUint64(buf, uint64(ts64))
    raw := make([]byte, 16)
    hex.Encode(raw, buf)
    ciphertext := make([]byte, aes.BlockSize + len(raw))
    iv := ciphertext[:aes.BlockSize]
    rand.Read(iv)
    stream := cipher.NewOFB(self.cipher, iv)
    stream.XORKeyStream(ciphertext[aes.BlockSize:], raw)
    return sippy_utils.B64EncodeNoPad(ciphertext)
}

func (self *AESCipher) Decrypt(enc string) (int64, error) {
    raw, err := sippy_utils.B64DecodeNoPad(enc)
    if err != nil {
        return 0, err
    }
    iv := raw[:aes.BlockSize]
    stream := cipher.NewOFB(self.cipher, iv)
    decrypted := make([]byte, 16)
    stream.XORKeyStream(decrypted, raw[aes.BlockSize:])
    buf := make([]byte, 8)
    if _, err := hex.Decode(buf, decrypted); err != nil {
        return 0, err
    }
    return int64(binary.BigEndian.Uint64(buf)), nil
}
