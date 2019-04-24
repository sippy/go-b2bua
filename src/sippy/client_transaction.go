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
    "sippy/net"
    "sippy/time"
    "sippy/types"
)

type clientTransaction struct {
    *baseTransaction
    teB             *Timeout
    teC             *Timeout
    teG             *Timeout
    r408            sippy_types.SipResponse
    resp_receiver   sippy_types.ResponseReceiver
    expires         time.Duration
    ack             sippy_types.SipRequest
    outbound_proxy  *sippy_net.HostPort
    cancel          sippy_types.SipRequest
    cancelPending   bool
    uack            bool
    ack_rAddr       *sippy_net.HostPort
    ack_checksum    string
    before_request_sent func(sippy_types.SipRequest)
    ack_rparams_present bool
    ack_rTarget     *sippy_header.SipURL
    ack_routes      []*sippy_header.SipRoute
    on_send_complete func()
    seen_rseqs      map[sippy_header.RTID]bool
}

func NewClientTransactionObj(req sippy_types.SipRequest, tid *sippy_header.TID, userv sippy_net.Transport, data []byte, sip_tm *sipTransactionManager, resp_receiver sippy_types.ResponseReceiver, session_lock sync.Locker, address *sippy_net.HostPort, req_out_cb func(sippy_types.SipRequest)) (*clientTransaction, error) {
    var r408 sippy_types.SipResponse = nil
    var err error

    if resp_receiver != nil {
        r408 = req.GenResponse(408, "Request Timeout", /*body*/ nil, /*server*/ nil)
    }
    expires := 32 * time.Second
    needack := false
    var ack, cancel sippy_types.SipRequest
    if req.GetMethod() == "INVITE" {
        expires = 300 * time.Second
        if req.GetExpires() != nil {
            exp, err := req.GetExpires().GetBody()
            if err == nil && exp.Number > 0 {
                expires = time.Duration(exp.Number) * time.Second
            }
        }
        needack = true
        if ack, err = req.GenACK(nil); err != nil {
            return nil, err
        }
        if cancel, err = req.GenCANCEL(); err != nil {
            return nil, err
        }
    }
    self := &clientTransaction{
        resp_receiver   : resp_receiver,
        cancelPending   : false,
        r408            : r408,
        expires         : expires,
        ack             : ack,
        cancel          : cancel,
        uack            : false,
        before_request_sent : req_out_cb,
        ack_rparams_present : false,
        seen_rseqs      : make(map[sippy_header.RTID]bool),
    }
    self.baseTransaction = newBaseTransaction(session_lock, tid, userv, sip_tm, address, data, needack)
    return self, nil
}

func (self *clientTransaction) SetOnSendComplete(fn func()) {
    self.on_send_complete = fn
}

func (self *clientTransaction) StartTimers() {
    self.startTeA()
    self.startTeB(32 * time.Second)
}

func (self *clientTransaction) cleanup() {
    self.baseTransaction.cleanup()
    self.ack = nil
    self.resp_receiver = nil
    if teB := self.teB; teB != nil { teB.Cancel(); self.teB = nil }
    if teC := self.teC; teC != nil { teC.Cancel(); self.teC = nil }
    if teG := self.teG; teG != nil { teG.Cancel(); self.teG = nil }
    self.r408 = nil
    self.cancel = nil
}

func (self *clientTransaction) SetOutboundProxy(outbound_proxy *sippy_net.HostPort) {
    self.outbound_proxy = outbound_proxy
}

func (self *clientTransaction) startTeC() {
    if teC := self.teC; teC != nil {
        teC.Cancel()
    }
    self.teC = StartTimeout(self.timerC, self.lock, 32 * time.Second, 1, self.logger)
}

func (self *clientTransaction) timerB() {
    if self.sip_tm == nil {
        return
    }
    //println("timerB", self.tid.String())
    self.cancelTeA()
    self.cancelTeB()
    self.state = TERMINATED
    self.startTeC()
    rtime, _ := sippy_time.NewMonoTime()
    if self.r408 != nil {
        self.r408.SetRtime(rtime)
    }
    if self.resp_receiver != nil {
        self.resp_receiver.RecvResponse(self.r408, self)
    }
}

func (self *clientTransaction) timerC() {
    if sip_tm := self.sip_tm; sip_tm != nil {
        sip_tm.tclient_del(self.tid)
        self.cleanup()
    }
}

func (self *clientTransaction) timerG() {
    if self.sip_tm == nil {
        return
    }
    self.teG = nil
    if self.state == UACK {
        self.logger.Error("INVITE transaction stuck in the UACK state, possible UAC bug")
    }
}

func (self *clientTransaction) cancelTeB() {
    if teB := self.teB; teB != nil {
        teB.Cancel()
        self.teB = nil
    }
}

func (self *clientTransaction) startTeB(timeout time.Duration) {
    if teB := self.teB; teB != nil {
        teB.Cancel()
    }
    self.teB = StartTimeout(self.timerB, self.lock, timeout, 1, self.logger)
}

func (self *clientTransaction) IncomingResponse(resp sippy_types.SipResponse, checksum string) {
    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    // In those two states upper level already notified, only do ACK retransmit
    // if needed
    if self.state == TERMINATED {
        return
    }
    code := resp.GetSCodeNum()
    if code > 100 && code < 200 && resp.GetRSeq() != nil {
        rskey := resp.GetRTId()
        if _, ok := self.seen_rseqs[*rskey]; ok {
            sip_tm.rcache_put(checksum, &sipTMRetransmitO{
                                userv : nil,
                                data  : nil,
                                address : nil,
                            })
            return
        }
        self.seen_rseqs[*rskey] = true
    }
    if self.state == TRYING {
        // Stop timers
        self.cancelTeA()
    }
    self.cancelTeB()
    if code < 200 {
        self.process_provisional_response(checksum, resp, sip_tm)
    } else {
        self.process_final_response(checksum, resp, sip_tm)
    }
}

func (self *clientTransaction) process_provisional_response(checksum string, resp sippy_types.SipResponse, sip_tm *sipTransactionManager) {
    // Privisional response - leave everything as is, except that
    // change state and reload timeout timer
    if self.state == TRYING {
        self.state = RINGING
        if self.cancelPending {
            sip_tm.BeginNewClientTransaction(self.cancel, nil, self.lock, nil, self.userv, self.before_request_sent)
            self.cancelPending = false
        }
    }
    self.startTeB(self.expires)
    sip_tm.rcache_set_call_id(checksum, self.tid.CallId)
    if self.resp_receiver != nil {
        self.resp_receiver.RecvResponse(resp, self)
    }
}

func (self *clientTransaction) process_final_response(checksum string, resp sippy_types.SipResponse, sip_tm *sipTransactionManager) {
    // Final response - notify upper layer and remove transaction
    if self.resp_receiver != nil {
        self.resp_receiver.RecvResponse(resp, self)
    }
    if self.needack {
        // Prepare and send ACK if necessary
        code := resp.GetSCodeNum()
        to_body, err := resp.GetTo().GetBody(sip_tm.config)
        if err != nil {
            self.logger.Debug(err.Error())
            return
        }
        tag := to_body.GetTag()
        if tag != "" {
            to_body, err = self.ack.GetTo().GetBody(sip_tm.config)
            if err != nil {
                self.logger.Debug(err.Error())
                return
            }
            to_body.SetTag(tag)
        }
        var rAddr *sippy_net.HostPort
        var rTarget *sippy_header.SipURL
        if code >= 200 && code < 300 {
            // Some hairy code ahead
            if len(resp.GetContacts()) > 0 {
                var contact *sippy_header.SipAddress
                contact, err = resp.GetContacts()[0].GetBody(sip_tm.config)
                if err != nil {
                    self.logger.Debug(err.Error())
                    return
                }
                rTarget = contact.GetUrl().GetCopy()
            } else {
                rTarget = nil
            }
            var routes []*sippy_header.SipRoute
            if ! self.ack_rparams_present {
                routes = make([]*sippy_header.SipRoute, len(resp.GetRecordRoutes()))
                for idx, r := range resp.GetRecordRoutes() {
                    r2 := r.AsSipRoute() // r.getCopy()
                    routes[len(resp.GetRecordRoutes()) - 1 - idx] = r2 // reverse order
                }
                if len(routes) > 0 {
                    var r0 *sippy_header.SipAddress
                    r0, err = routes[0].GetBody(sip_tm.config)
                    if err != nil {
                        self.logger.Debug(err.Error())
                        return
                    }
                    if ! r0.GetUrl().Lr {
                        if rTarget != nil {
                            routes = append(routes, sippy_header.NewSipRoute(sippy_header.NewSipAddress("", rTarget)))
                        }
                        rTarget = r0.GetUrl()
                        routes = routes[1:]
                        rAddr = rTarget.GetAddr(sip_tm.config)
                    } else {
                        rAddr = r0.GetUrl().GetAddr(sip_tm.config)
                    }
                } else if rTarget != nil {

                    rAddr = rTarget.GetAddr(sip_tm.config)
                }
                if rTarget != nil {
                    self.ack.SetRURI(rTarget)
                }
                if self.outbound_proxy != nil {
                    routes = append([]*sippy_header.SipRoute{ sippy_header.NewSipRoute(sippy_header.NewSipAddress("", sippy_header.NewSipURL("", self.outbound_proxy.Host, self.outbound_proxy.Port, true))) }, routes...)
                    rAddr = self.outbound_proxy
                }
            } else {
                rAddr, rTarget, routes = self.ack_rAddr, self.ack_rTarget, self.ack_routes
            }
            self.ack.SetRoutes(routes)
        }
        if code >= 200 && code < 300 {
            var via0 *sippy_header.SipViaBody
            if via0, err = self.ack.GetVias()[0].GetBody(); err != nil {
                self.logger.Debug("error parsing via: " + err.Error())
                return
            }
            via0.GenBranch()
        }
        if rAddr == nil {
            rAddr = self.address
        }
        if ! self.uack {
            self.BeforeRequestSent(self.ack)
            sip_tm.transmitMsg(self.userv, self.ack, rAddr, checksum, self.tid.CallId)
        } else {
            self.state = UACK
            self.ack_rAddr = rAddr
            self.ack_checksum = checksum
            sip_tm.rcache_set_call_id(checksum, self.tid.CallId)
            self.teG = StartTimeout(self.timerG, self.lock, 64 * time.Second, 1, self.logger)
            return
        }
    } else {
        sip_tm.rcache_set_call_id(checksum, self.tid.CallId)
    }
    sip_tm.tclient_del(self.tid)
    self.cleanup()
}

func (self *clientTransaction) Cancel(extra_headers ...sippy_header.SipHeader) {
    sip_tm := self.sip_tm
    if sip_tm == nil {
        return
    }
    // If we got at least one provisional reply then (state == RINGING)
    // then start CANCEL transaction, otherwise deffer it
    if self.state != RINGING {
        self.cancelPending = true
    } else {
        if extra_headers != nil {
            for _, h := range extra_headers {
                self.cancel.AppendHeader(h)
            }
        }
        sip_tm.BeginNewClientTransaction(self.cancel, nil, self.lock, nil, self.userv, self.before_request_sent)
    }
}

func (self *clientTransaction) Lock() {
    self.lock.Lock()
}

func (self *clientTransaction) Unlock() {
    self.lock.Unlock()
}

func (self *clientTransaction) SendACK() {
    if teG := self.teG; teG != nil {
        teG.Cancel()
        self.teG = nil
    }
    self.BeforeRequestSent(self.ack)
    if sip_tm := self.sip_tm; sip_tm != nil {
        sip_tm.transmitMsg(self.userv, self.ack, self.ack_rAddr, self.ack_checksum, self.tid.CallId)
        sip_tm.tclient_del(self.tid)
    }
    self.cleanup()
}

func (self *clientTransaction) GetACK() sippy_types.SipRequest {
    return self.ack
}

func (self *clientTransaction) SetUAck(uack bool) {
    self.uack = uack
}

func (self *clientTransaction) BeforeRequestSent(req sippy_types.SipRequest) {
    if self.before_request_sent != nil {
        self.before_request_sent(req)
    }
}

func (self *clientTransaction) TransmitData() {
    if sip_tm := self.sip_tm; sip_tm != nil {
        sip_tm.transmitDataWithCb(self.userv, self.data, self.address, /*cachesum*/ "", /*call_id =*/ self.tid.CallId, 0, self.on_send_complete)
    }
}

func (self *clientTransaction) SetAckRparams(rAddr *sippy_net.HostPort, rTarget *sippy_header.SipURL, routes []*sippy_header.SipRoute) {
    self.ack_rparams_present = true
    self.ack_rAddr = rAddr
    self.ack_rTarget = rTarget
    self.ack_routes = routes
}
