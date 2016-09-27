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
    "math/rand"
    "time"
    "sync"

    "sippy/log"
    "sippy/utils"
)

type Timeout struct {
    callback        func()
    timeout         time.Duration
    logger          sippy_log.ErrorLogger
    shutdown_chan   chan struct{}
    shutdown        bool
    spread          time.Duration
    nticks          int
    lock            sync.Mutex
    cb_lock         sync.Locker
    started         bool
}

func StartTimeout(callback func(), cb_lock sync.Locker, _timeout time.Duration, nticks int, logger sippy_log.ErrorLogger) *Timeout {
    self := NewInactiveTimeout(callback, cb_lock, _timeout, nticks, logger)
    self.Start()
    return self
}

func NewInactiveTimeout(callback func(), cb_lock sync.Locker, _timeout time.Duration, nticks int, logger sippy_log.ErrorLogger) *Timeout {
    self := &Timeout{
        callback        : callback,
        timeout         : _timeout,
        nticks          : nticks,
        logger          : logger,
        shutdown_chan   : make(chan struct{}),
        shutdown        : false,
        spread          : 0,
        started         : false,
        cb_lock         : cb_lock,
    }
    return self
}

func (self *Timeout) Start() {
    self.lock.Lock()
    if ! self.started && self.callback != nil {
        self.started = true
        go self.run()
    }
    self.lock.Unlock()
}

func (self *Timeout) SpreadRuns(spread time.Duration) {
    self.spread = spread
}

func (self *Timeout) Cancel() {
    self.shutdown = true
    close(self.shutdown_chan)
}

func (self *Timeout) run() {
    for !self.shutdown {
        self._run()
    }
    self.callback = nil
    self.cb_lock = nil
}

func (self *Timeout) _run() {
    for !self.shutdown {
        if self.nticks == 0 {
            self.shutdown = true
            break
        }
        if self.nticks > 0 {
            self.nticks--
        }
        t := self.timeout
        if self.spread > 0 {
            t += time.Duration(float64(self.spread) * (rand.Float64() - 0.5))
        }
        select {
        case <-self.shutdown_chan:
            self.shutdown = true
        case <-time.After(t):
            sippy_utils.SafeCall(self.callback, self.cb_lock, self.logger)
        }
    }
}
