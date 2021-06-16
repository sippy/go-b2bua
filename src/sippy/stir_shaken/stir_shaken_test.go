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
    "testing"
    "time"
)

func TestStirShaken(t *testing.T) {
    attest := "C"
    origid := "cafdc332-e152-11ea-b360-080027e00f8a"
    cert_str := `-----BEGIN CERTIFICATE-----
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
    pkey_str := `-----BEGIN EC PARAMETERS-----
BggqhkjOPQMBBw==
-----END EC PARAMETERS-----
-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIPdbKAmBh4ucVwNAlbYG/gH/irbtC+p3PV+Cmrf9abHxoAoGCCqGSM49
AwEHoUQDQgAENNTB4FbGbt/GeZ5dNW24vDNiV12O/UNrdN6FEobikGjfseLqgThG
uoBgT+V+ZUUHF8HSpAy4Ey2BC73Gdj1Fpg==
-----END EC PRIVATE KEY-----
`
    cr_url := "https://certs.example.org/cert.pem"
    orig_tn := "12345678901"
    dest_tn := "12345678902"
    cert_buf := []byte(cert_str)
    pkey_buf := []byte(pkey_str)
    date_ts := time.Now()
    root_certs := `-----BEGIN CERTIFICATE-----
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
-----BEGIN CERTIFICATE-----
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
    chain_buf := []byte(root_certs)
    verifier, err := NewVerifier(chain_buf)
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
