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
    "hash"
    "crypto/md5"
    "crypto/sha256"
    "crypto/sha512"
)

type Algorithm struct {
    Hash        hash.Hash
    Mask        int64
}
const (
    DGST_MD5        = int64(1 << 0)
    DGST_MD5SESS    = int64(1 << 1)
    DGST_SHA256     = int64(1 << 2)
    DGST_SHA256SESS = int64(1 << 3)
    DGST_SHA512     = int64(1 << 4)
    DGST_SHA512SESS = int64(1 << 5)

    NUM_OF_DGSTS    = 6
)

var algorithms = map[string]*Algorithm{
    ""                  : &Algorithm{ md5.New(), DGST_MD5 },
    "MD5"               : &Algorithm{ md5.New(), DGST_MD5 },
    "MD5-sess"          : &Algorithm{ md5.New(), DGST_MD5SESS },
    "SHA-256"           : &Algorithm{ sha256.New(), DGST_SHA256 },
    "SHA-256-sess"      : &Algorithm{ sha256.New(), DGST_SHA256SESS },
    "SHA-512-256"       : &Algorithm{ sha512.New512_256(), DGST_SHA512 },
    "SHA-512-256-sess"  : &Algorithm{ sha512.New512_256(), DGST_SHA512SESS },
}

func GetAlgorithm(alg_name string) *Algorithm {
    return algorithms[alg_name]
}
