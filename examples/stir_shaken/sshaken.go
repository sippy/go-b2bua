//
// Copyright (c) 2006-2021 Sippy Software, Inc. All rights reserved.
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

package main

import (
    "errors"
    "io"
    "net/http"
    "os"
    "time"

    "sippy/stir_shaken"
)

type StirShaken struct {
    verifier    sippy_sshaken.Verifier
    config      *myconfig
    cert_buf    []byte
    pkey_buf    []byte
}

func NewStirShaken(config *myconfig) (*StirShaken, error) {
    chain_buf, err := read_file(config.crt_roots_file)
    if err != nil {
        return nil, err
    }
    cert_buf, err := read_file(config.crt_file)
    if err != nil {
        return nil, err
    }
    pkey_buf, err := read_file(config.pkey_file)
    if err != nil {
        return nil, err
    }
    ret := &StirShaken{
        config      : config,
        cert_buf    : cert_buf,
        pkey_buf    : pkey_buf,
    }
    ret.verifier, err = sippy_sshaken.NewVerifier(chain_buf)
    if err != nil {
        return nil, err
    }
    return ret, nil
}

func (self *StirShaken) Authenticate(date_ts time.Time, cli, cld string) (string, error) {
    return sippy_sshaken.Authenticate(date_ts, self.config.attest, self.config.origid, self.cert_buf, self.pkey_buf, self.config.x5u, cli, cld)
}

func (self *StirShaken) Verify(identity, orig_tn, dest_tn string, date_ts time.Time) error {
    passport, err := sippy_sshaken.ParseIdentity(identity)
    if err != nil {
        return err
    }
    cert_buf, err := self.GetCert(passport.Header.X5u)
    if err != nil {
        return err
    }
    return self.verifier.Verify(passport, cert_buf, orig_tn, dest_tn, date_ts)
}

func (self *StirShaken) GetCert(url string) ([]byte, error) {
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    cert, err := read_all(resp.Body)
    if err != nil {
        return nil, err
    }
    if len(cert) == 0 {
        return nil, errors.New("Empty certificate retrieved")
    }
    return cert, nil
}

func read_all(fd io.Reader) ([]byte, error) {
    buf := make([]byte, 4096)
    n, err := fd.Read(buf)
    if err != nil && err != io.EOF {
        return nil, err
    }
    if n == 0 {
        return nil, errors.New("Empty certificate retrieved")
    }
    return buf[:n], nil
}

func read_file(fname string) ([]byte, error) {
    fd, err := os.Open(fname)
    if err != nil {
        return nil, err
    }
    defer fd.Close()
    return read_all(fd)
}
