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
    "crypto/elliptic"
    "crypto/rand"
    "crypto/x509"
    "crypto/x509/pkix"
    "encoding/pem"
    "math/big"
    "strings"
    "testing"
    "time"
)

// The original hardcoded fixture certificate (valid 2020-08-18..2021-08-18) is
// kept as an EXPIRED negative fixture: production validity checks must reject
// it. The happy path uses a test-generated certificate valid around the
// current time, because Authenticate checks both the actual time.Now()
// (freshness and validity) and the supplied Date, so no fixed historical
// timestamp can satisfy it.
const legacyExpiredCertStr = `-----BEGIN CERTIFICATE-----
MIIBnjCCAUOgAwIBAgIJAIQSOKHaTrn2MAoGCCqGSM49BAMCMEUxCzAJBgNVBAYT
AkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEwHwYDVQQKDBhJbnRlcm5ldCBXaWRn
aXRzIFB0eSBMdGQwHhcNMjAwODE4MTYwMzU3WhcNMjEwODE4MTYwMzU3WjBFMQsw
CQYDVQQGEwJBVTETMBEGA1UECAwKU29tZS1TdGF0ZTEhMB8GA1UECgwYSW50ZXJu
ZXQgV2lkZ2l0cyBQdHkgTHRkMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAENNTB
4FbGbt/GeZ5dNW24vDNiV12O/UNrdN6FEobikGjfseLqgThGuoBgT+V+ZUUHF8HS
pAy4Ey2BC73Gdj1FpqMcMBowGAYIKwYBBQUHARoEDAwKVE5BdXRoTGlzdDAKBggq
hkjOPQQDAgNJADBGAiEAoRvsrcKrd2ajDQp2o010BygDkUT9pW93svK/ikFn3UUC
IQCdCcoA3EGt/qsLGqUQjukv6bgIwxPVCbx7mgCJSNRBDw==
-----END CERTIFICATE-----
`

const legacyExpiredPkeyStr = `-----BEGIN EC PARAMETERS-----
BggqhkjOPQMBBw==
-----END EC PARAMETERS-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIPdbKAmBh4ucVwNAlbYG/gH/irbtC+p3PV+Cmrf9abHxoAoGCCqGSM49
AwEHoUQDQgAENNTB4FbGbt/GeZ5dNW24vDNiV12O/UNrdN6FEobikGjfseLqgThG
uoBgT+V+ZUUHF8HSpAy4Ey2BC73Gdj1Fpg==
-----END EC PRIVATE KEY-----
`

// genTestCert generates a test-only EC P-256 self-signed certificate with the
// TnAuthList extension required by the production verifier. It returns the
// certificate PEM (suitable as its own root) and the private key PEM.
func genTestCert(t *testing.T, notBefore, notAfter time.Time) (certPEM, keyPEM []byte) {
    t.Helper()
    key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        t.Fatalf("key generation: %v", err)
    }
    tmpl := x509.Certificate{
        SerialNumber:          big.NewInt(0x0123456789abcdef),
        Subject:               pkix.Name{Country: []string{"AU"}, Province: []string{"Some-State"}, Organization: []string{"Internet Widgits Pty Ltd"}},
        NotBefore:             notBefore,
        NotAfter:              notAfter,
        KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
        BasicConstraintsValid: true,
        IsCA:                  true,
        ExtraExtensions: []pkix.Extension{{
            Id:    TNAUTHLIST_EXT,
            Value: []byte{0x02, 0x00, 0x00}, // empty TnAuthList, same as the legacy fixture
        }},
    }
    der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
    if err != nil {
        t.Fatalf("certificate creation: %v", err)
    }
    keyDER, err := x509.MarshalECPrivateKey(key)
    if err != nil {
        t.Fatalf("key marshaling: %v", err)
    }
    certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
    keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
    return
}

func TestStirShaken(t *testing.T) {
    attest := "C"
    origid := "cafdc332-e152-11ea-b360-080027e00f8a"
    cr_url := "https://certs.example.org/cert.pem"
    orig_tn := "12345678901"
    dest_tn := "12345678902"

    now := time.Now()
    cert_buf, pkey_buf := genTestCert(t, now.Add(-24*time.Hour), now.Add(24*time.Hour))
    date_ts := time.Now()

    verifier, err := NewVerifier(cert_buf)
    if err != nil {
        t.Error(err.Error())
        return
    }
    identity, err := Authenticate(date_ts, attest, origid, cert_buf, pkey_buf, cr_url, orig_tn, dest_tn)
    if err != nil {
        t.Error(err.Error())
        return
    }
    passport, err := ParseIdentity(identity)
    if err != nil {
        t.Error(err.Error())
        return
    }
    err = verifier.Verify(passport, cert_buf, orig_tn, dest_tn, date_ts)
    if err != nil {
        t.Error(err.Error())
        return
    }
}

// TestStirShakenExpiredCert: the legacy 2020-2021 fixture must be rejected by
// the production certificate validity check (expired relative to time.Now()).
func TestStirShakenExpiredCert(t *testing.T) {
    date_ts := time.Now()
    _, err := Authenticate(date_ts, "C", "cafdc332-e152-11ea-b360-080027e00f8a",
        []byte(legacyExpiredCertStr), []byte(legacyExpiredPkeyStr),
        "https://certs.example.org/cert.pem", "12345678901", "12345678902")
    if err == nil {
        t.Fatal("expired certificate was accepted")
    }
    if !strings.Contains(err.Error(), "outside the certificate validity") {
        t.Errorf("unexpected error for expired cert: %v", err)
    }
}

// TestStirShakenNotYetValidCert: a certificate whose validity starts in the
// future must be rejected by the production validity check.
func TestStirShakenNotYetValidCert(t *testing.T) {
    now := time.Now()
    cert_buf, pkey_buf := genTestCert(t, now.Add(1*time.Hour), now.Add(24*time.Hour))
    _, err := Authenticate(time.Now(), "C", "cafdc332-e152-11ea-b360-080027e00f8a",
        cert_buf, pkey_buf,
        "https://certs.example.org/cert.pem", "12345678901", "12345678902")
    if err == nil {
        t.Fatal("not-yet-valid certificate was accepted")
    }
    if !strings.Contains(err.Error(), "outside the certificate validity") {
        t.Errorf("unexpected error for not-yet-valid cert: %v", err)
    }
}

// TestStirShakenStaleDateAuth: a Date older than the 60s freshness policy must
// be rejected by Authenticate even with a valid certificate.
func TestStirShakenStaleDateAuth(t *testing.T) {
    now := time.Now()
    cert_buf, pkey_buf := genTestCert(t, now.Add(-24*time.Hour), now.Add(24*time.Hour))
    _, err := Authenticate(now.Add(-120*time.Second), "C", "cafdc332-e152-11ea-b360-080027e00f8a",
        cert_buf, pkey_buf,
        "https://certs.example.org/cert.pem", "12345678901", "12345678902")
    if err == nil {
        t.Fatal("stale Date was accepted by Authenticate")
    }
    if !strings.Contains(err.Error(), "older than local policy") {
        t.Errorf("unexpected error for stale date: %v", err)
    }
}

// TestStirShakenStaleDateVerify: a stale Date must be rejected by Verify even
// when the passport itself was generated with a fresh timestamp.
func TestStirShakenStaleDateVerify(t *testing.T) {
    now := time.Now()
    cert_buf, pkey_buf := genTestCert(t, now.Add(-24*time.Hour), now.Add(24*time.Hour))
    verifier, err := NewVerifier(cert_buf)
    if err != nil {
        t.Fatal(err.Error())
    }
    fresh_ts := time.Now()
    identity, err := Authenticate(fresh_ts, "C", "cafdc332-e152-11ea-b360-080027e00f8a",
        cert_buf, pkey_buf,
        "https://certs.example.org/cert.pem", "12345678901", "12345678902")
    if err != nil {
        t.Fatal(err.Error())
    }
    passport, err := ParseIdentity(identity)
    if err != nil {
        t.Fatal(err.Error())
    }
    stale_ts := fresh_ts.Add(-120 * time.Second)
    err = verifier.Verify(passport, cert_buf, "12345678901", "12345678902", stale_ts)
    if err == nil {
        t.Fatal("stale Date was accepted by Verify")
    }
    if !strings.Contains(err.Error(), "older than local policy") {
        t.Errorf("unexpected error for stale date: %v", err)
    }
}

// TestStirShakenTamperedSignature: a flipped signature byte must fail the
// production signature check.
func TestStirShakenTamperedSignature(t *testing.T) {
    now := time.Now()
    cert_buf, pkey_buf := genTestCert(t, now.Add(-24*time.Hour), now.Add(24*time.Hour))
    verifier, err := NewVerifier(cert_buf)
    if err != nil {
        t.Fatal(err.Error())
    }
    date_ts := time.Now()
    identity, err := Authenticate(date_ts, "C", "cafdc332-e152-11ea-b360-080027e00f8a",
        cert_buf, pkey_buf,
        "https://certs.example.org/cert.pem", "12345678901", "12345678902")
    if err != nil {
        t.Fatal(err.Error())
    }
    // identity: <b64hdr>.<b64payload>.<b64sig>;info=<...>;ppt=shaken
    parts := strings.SplitN(identity, ".", 3)
    if len(parts) != 3 {
        t.Fatalf("unexpected identity format: %s", identity)
    }
    sig := []byte(parts[2])
    if sig[0] == 'A' {
        sig[0] = 'B'
    } else {
        sig[0] = 'A'
    }
    parts[2] = string(sig)
    passport, err := ParseIdentity(strings.Join(parts, "."))
    if err != nil {
        t.Fatal(err.Error())
    }
    err = verifier.Verify(passport, cert_buf, "12345678901", "12345678902", date_ts)
    if err == nil {
        t.Fatal("tampered signature was accepted")
    }
}
