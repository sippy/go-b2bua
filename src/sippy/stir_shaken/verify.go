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
    "crypto/sha256"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "encoding/pem"
    "errors"
    "fmt"
    "io"
    "math/big"
    "os"
    "strings"
    "time"
)

type sshaken_verifier struct {
    roots       *x509.CertPool
}

func NewStirVerifier(roots_file string) (*sshaken_verifier, error) {
    fd, err := os.Open(roots_file)
    if err != nil {
        return nil, err
    }
    self := &sshaken_verifier{
        roots   : x509.NewCertPool(),
    }
    buf := make([]byte, 16384)
    chain_buf := []byte{}
    for {
        n, err := fd.Read(buf)
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }
        chain_buf = append(chain_buf, buf[:n]...)
    }
    if ! self.roots.AppendCertsFromPEM(chain_buf) {
        return nil, errors.New("error parsing the root certificates")
    }
    return self, nil
}

func (self *sshaken_verifier) Verify(identity string, cert_buf, chain_buf []byte, orig_tn_p, dest_tn_p string, date_ts time.Time) error {
    parsed, err := parse_identity_hf(identity)
    if err != nil {
        return err
    }
    if parsed.ppt_hdr_param != "shaken"{
        return errors.New("Unsupported 'ppt' extension")
    }
    if parsed.alg_hdr_param != "" && parsed.alg_hdr_param != "ES256" {
        return errors.New("Unsupported 'alg'")
    }
    err = parsed.check_claims()
    if err != nil {
        return err
    }
    now := time.Now()
    if now.Sub(date_ts) > VERIFY_DATE_FRESHNESS {
        return errors.New("Date header value is older than local policy")
    }
    if parsed.OrigTN() != orig_tn_p || parsed.DestTN() != dest_tn_p {
        return errors.New("Signature would not verify successfully")
    }
    cert, err := load_cert(cert_buf, chain_buf)
    if err != nil {
        return err
    }
    check_cert_validity(cert, date_ts)
    err = self.validate_certificate(cert)
    if err != nil {
        return err
    }
    iat_ts := parsed.Iat()
    if iat_ts != date_ts && now.Sub(iat_ts) > VERIFY_DATE_FRESHNESS {
        iat_ts = date_ts
    }
    return verify_signature(cert, parsed, iat_ts, orig_tn_p, dest_tn_p)
}

func (self *sshaken_verifier) validate_certificate(cert *x509.Certificate) error {
    tn_ext_found := false
    for _, ext := range cert.Extensions {
        if ext.Id.Equal(TNAUTHLIST_EXT) {
            tn_ext_found = true
            break
        }
    }
    if ! tn_ext_found {
        return errors.New("The certificate misses TnAuthList extension")
    }
    opts := x509.VerifyOptions{
        Roots: self.roots,
    }
    chains, err := cert.Verify(opts)
    if err != nil {
        return err
    }
    if len(chains) == 0 {
        return errors.New("no matching root certificate")
    }
    return nil
}

func build_unsigned_pport(iat_ts time.Time, attest, cr_url, orig_tn, dest_tn, origid string) string {
    hdr := sshaken_header{
        Alg : "ES256",
        Ppt : "shaken",
        Typ : "passport",
        X5u : cr_url,
    }
    hdr_json_str, _ := json.Marshal(hdr)

    payload := sshaken_payload{
        Attest  : attest,
        Dest    : sshaken_dest{
            TN  : []string{ dest_tn },
        },
        Iat     : iat_ts.Unix(),
        Orig    : sshaken_orig{
            TN  : orig_tn,
        },
        Origid  : origid,
    }
    payload_json_str, _ := json.Marshal(payload)
    return strings.TrimRight(base64.URLEncoding.EncodeToString(hdr_json_str), "=") + "." +
        strings.TrimRight(base64.URLEncoding.EncodeToString(payload_json_str), "=")
}

func verify_signature(cert *x509.Certificate, parsed *sshaken_passport, iat_ts time.Time, orig_tn, dest_tn string) error {
    if len(parsed.signature) != 64 {
        return fmt.Errorf("Bad raw signature length %d, should be 64", len(parsed.signature))
    }
    unsigned_buf := build_unsigned_pport(iat_ts, parsed.Attest(), parsed.X5u(), orig_tn, dest_tn, parsed.Origid())
    r := big.NewInt(0).SetBytes(parsed.signature[:32])
    s := big.NewInt(0).SetBytes(parsed.signature[32:])
    hash := sha256.Sum256([]byte(unsigned_buf))
    pub_key, ok := cert.PublicKey.(*ecdsa.PublicKey)
    if ! ok {
        return errors.New("wrong public key")
    }
    if ! ecdsa.Verify(pub_key, hash[:], r, s) {
        return errors.New("verification failed")
    }
    return nil
}

func load_cert(cert_buf, chain_buf []byte) (*x509.Certificate, error) {
    block, _ := pem.Decode(cert_buf)
    cert, err := x509.ParseCertificate(block.Bytes)
    if err != nil {
        return nil, err
    }
    return cert, nil
}

func parse_identity_hf(hdr_buf string) (*sshaken_passport, error) {
    arr := strings.SplitN(hdr_buf, ";", 2)
    if len(arr) != 2 {
        return nil, errors.New("no parameters in Identity")
    }
    hdr_arr := strings.Split(arr[0], ".")
    if len(hdr_arr) != 3 {
        return nil, errors.New("Bad signature")
    }
    params_arr := strings.Split(arr[1], ";")
    if len(params_arr) == 0 {
        return nil, errors.New("Header parameters missing")
    }
    parsed := &sshaken_passport{}

    for _, param := range params_arr {
        p_arr := strings.SplitN(param, "=", 2)
        if len(p_arr) != 2 {
            continue
        }
        switch p_arr[0] {
        case "alg":
            parsed.alg_hdr_param = strings.Trim(p_arr[1], "\"")
        case "ppt":
            parsed.ppt_hdr_param = strings.Trim(p_arr[1], "\"")
        }
    }
    buf, err := b64decode_nopad(hdr_arr[0])
    if err != nil {
        return nil, err
    }
    if err = json.Unmarshal(buf, &parsed.header); err != nil {
        return nil, err
    }
    buf, err = b64decode_nopad(hdr_arr[1])
    if err != nil {
        return nil, err
    }
    if err = json.Unmarshal(buf, &parsed.payload); err != nil {
        return nil, err
    }
    parsed.signature, err = b64decode_nopad(hdr_arr[2])
    if err != nil {
        return nil, err
    }
    return parsed, nil
}

func b64decode_nopad(s string) ([]byte, error) {
    if l := len(s) % 4; l > 0 {
        s += strings.Repeat("=", 4 - l)
    }
    return base64.URLEncoding.DecodeString(s)
}
