// Copyright (c) 2003-2005 Maxim Sobolev. All rights reserved.
// Copyright (c) 2006-2015 Sippy Software, Inc. All rights reserved.
// Copyright (c) 2015 Andrii Pylypenko. All rights reserved.
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
package sippy

import (
    "sync"
    "time"

    "sippy/headers"
    "sippy/log"
    "sippy/net"
)

type sip_transaction_state int

const (
    TRYING = sip_transaction_state(iota)
    RINGING
    COMPLETED
    CONFIRMED
    TERMINATED
    UACK
)

func (self sip_transaction_state) String() string {
    switch self {
    case TRYING:        return "TRYING"
    case RINGING:       return "RINGING"
    case COMPLETED:     return "COMPLETED"
    case CONFIRMED:     return "CONFIRMED"
    case TERMINATED:    return "TERMINATED"
    default:            return "UNKNOWN"
    }
}

type baseTransaction struct {
    lock            sync.Locker
    userv           sippy_net.Transport
    sip_tm          *sipTransactionManager
    state           sip_transaction_state
    tid             *sippy_header.TID
    teA             *Timeout
    address         *sippy_net.HostPort
    needack         bool
    tout            time.Duration
    data            []byte
    logger          sippy_log.ErrorLogger
}

func newBaseTransaction(lock sync.Locker, tid *sippy_header.TID, userv sippy_net.Transport, sip_tm *sipTransactionManager, address *sippy_net.HostPort, data []byte, needack bool) *baseTransaction {
    return &baseTransaction{
        tout    : time.Duration(0.5 * float64(time.Second)),
        userv   : userv,
        tid     : tid,
        state   : TRYING,
        sip_tm  : sip_tm,
        address : address,
        data    : data,
        needack : needack,
        lock    : lock,
        logger  : sip_tm.config.ErrorLogger(),
    }
}

func (self *baseTransaction) cleanup() {
    self.sip_tm = nil
    self.userv = nil
    self.tid = nil
    self.address = nil
    if self.teA != nil { self.teA.Cancel(); self.teA = nil }
}

func (self *baseTransaction) cancelTeA() {
    if self.teA != nil {
        self.teA.Cancel()
        self.teA = nil
    }
}

func (self *baseTransaction) startTeA() {
    if self.teA != nil {
        self.teA.Cancel()
    }
    self.teA = StartTimeout(self.timerA, self.lock, self.tout, 1, self.logger)
}

func (self *baseTransaction) timerA() {
    //print("timerA", t.GetTID())
    if sip_tm := self.sip_tm; sip_tm != nil {
        sip_tm.transmitData(self.userv, self.data, self.address, /*cachesum*/ "", /*call_id*/ self.tid.CallId, 0)
        self.tout *= 2
        self.teA = StartTimeout(self.timerA, self.lock, self.tout, 1, self.logger)
    }
}

func (self *baseTransaction) GetHost() string {
    return self.address.Host.String()
}
