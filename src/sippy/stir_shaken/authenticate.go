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
package sippy_sshaken

import (
    "crypto/ecdsa"
    "crypto/rand"
    "crypto/sha256"
    "crypto/x509"
    "encoding/asn1"
    "encoding/pem"
    "errors"
    "time"

    "sippy/utils"
)

func Authenticate(date_ts time.Time, attest, origid string, cert_buf, pkey_buf []byte, cr_url, orig_tn, dest_tn string) (string, error) {
    now := time.Now()
    if now.Sub(date_ts) > AUTH_DATE_FRESHNESS {
        return "", errors.New("Date header value is older than local policy")
    }
    cert, err := load_certificate(cert_buf)
    if err != nil {
        return "", err
    }
    pkey, err := load_privatekey(pkey_buf)
    if err != nil {
        return "", err
    }
    if err = check_cert_validity(cert, now); err != nil {
        return "", err
    }
    if err = check_cert_validity(cert, date_ts); err != nil {
        return "", err
    }
    return gen_identity(pkey, date_ts, attest, cr_url, orig_tn, dest_tn, origid)
}

func load_certificate(cert_buf []byte) (*x509.Certificate, error) {
    block, _ := pem.Decode(cert_buf)
    if block == nil {
        return nil, errors.New("cannot decode certificate")
    }
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return nil, err
    }
    return cert, nil
}

func load_privatekey(pkey_buf []byte) (*ecdsa.PrivateKey, error) {
    var block *pem.Block

    for {
        block, pkey_buf = pem.Decode(pkey_buf)
        if block == nil {
            return nil, errors.New("cannot decode private key")
        }
        switch block.Type {
        case "EC PRIVATE KEY":
            pkey, err := x509.ParseECPrivateKey(block.Bytes)
            if err != nil {
                return nil, err
            }
            return pkey, nil
        case "EC PARAMETERS":
            obj := make(asn1.ObjectIdentifier, 7)
            _, err := asn1.Unmarshal(block.Bytes, &obj)
            if err != nil {
                return nil, err
            }
            if ! obj.Equal(EC_P256_PARAMS) {
                return nil, errors.New("Wrong EC params")
            }
        default:
            return nil, errors.New("bad private key")
        }
    }
}

func check_cert_validity(cert *x509.Certificate, date_ts time.Time) error {
    if date_ts.Before(cert.NotBefore) || date_ts.After(cert.NotAfter) {
        return errors.New("Date is outside the certificate validity")
    }
    return nil
}

func gen_identity(pkey *ecdsa.PrivateKey, date_ts time.Time, attest, cr_url, orig_tn, dest_tn, origid string) (string, error) {
    unsigned_buf := build_unsigned_pport(date_ts, attest, cr_url, orig_tn, dest_tn, origid)
    hash := sha256.Sum256([]byte(unsigned_buf))
    r, s, err := ecdsa.Sign(rand.Reader, pkey, hash[:])
    if err != nil {
        return "", err
    }
    r_padded := make([]byte, 32)
    copy(r_padded[32 - len(r.Bytes()):], r.Bytes())
    s_padded := make([]byte, 32)
    copy(s_padded[32 - len(s.Bytes()):], s.Bytes())
    buf := sippy_utils.B64EncodeNoPad(append(r_padded, s_padded...))
    identity := unsigned_buf + "." + buf + ";info=<" + cr_url + ">;ppt=shaken"
    return identity, nil
}
