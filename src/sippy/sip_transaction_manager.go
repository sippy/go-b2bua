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
    "crypto/md5"
    "errors"
    "fmt"
    "net"
    "strings"
    "sync"
    "time"

    "sippy/conf"
    "sippy/headers"
    "sippy/net"
    "sippy/types"
    "sippy/time"
    "sippy/utils"
)

type sipTransactionManager struct {
    call_map        sippy_types.CallMap
    l4r             *local4remote
    l1rcache        map[string]*sipTMRetransmitO
    l2rcache        map[string]*sipTMRetransmitO
    rcache_lock     sync.Mutex
    shutdown_chan   chan int
    config          sippy_conf.Config
    tclient         map[sippy_header.TID]sippy_types.ClientTransaction
    tclient_lock    sync.Mutex
    tserver         map[sippy_header.TID]sippy_types.ServerTransaction
    tserver_lock    sync.Mutex
    nat_traversal   bool
    req_consumers   map[string][]sippy_types.UA
    consumers_lock  sync.Mutex
    pass_t_to_cb    bool
    provisional_retr time.Duration
    before_response_sent func(sippy_types.SipResponse)
    rtid2tid        map[sippy_header.RTID]*sippy_header.TID
    rtid2tid_lock   sync.Mutex
}

type sipTMRetransmitO struct {
    userv       sippy_net.Transport
    data        []byte
    address     *sippy_net.HostPort
    call_id     string
    lossemul    int
}

func NewSipTransactionManager(config sippy_conf.Config, call_map sippy_types.CallMap) (*sipTransactionManager, error) {
    var err error

    self := &sipTransactionManager{
        call_map        : call_map,
        l1rcache        : make(map[string]*sipTMRetransmitO),
        l2rcache        : make(map[string]*sipTMRetransmitO),
        shutdown_chan   : make(chan int),
        config          : config,
        tclient         : make(map[sippy_header.TID]sippy_types.ClientTransaction),
        tserver         : make(map[sippy_header.TID]sippy_types.ServerTransaction),
        nat_traversal   : false,
        req_consumers   : make(map[string][]sippy_types.UA),
        pass_t_to_cb    : false,
        provisional_retr : 0,
        rtid2tid        : make(map[sippy_header.RTID]*sippy_header.TID),
    }
    self.l4r, err = NewLocal4Remote(config, self.handleIncoming)
    if err != nil {
        return nil, err
    }
    go func() {
        for {
            time.Sleep(32 * time.Second)
            self.rCachePurge()
        }
    }()
    return self, nil
}

func (self *sipTransactionManager) Run() {
    <-self.shutdown_chan
    self.l4r.shutdown()
}

func (self *sipTransactionManager) rCachePurge() {
    self.rcache_lock.Lock()
    defer self.rcache_lock.Unlock()
    self.l2rcache = self.l1rcache
    self.l1rcache = make(map[string]*sipTMRetransmitO)
    self.l4r.rotateCache()
}

var NETS_1918 = []struct {
                    ip net.IP
                    mask net.IPMask
                } {
        { net.IPv4(10,0,0,0), net.IPv4Mask(255,0,0,0) },
        { net.IPv4(172,16,0,0), net.IPv4Mask(255,240,0,0) },
        { net.IPv4(192,168,0,0), net.IPv4Mask(255,255,0,0) },
    }

func check1918(host string) bool {
    ip := net.ParseIP(host)
    if ip == nil { return false }
    if ip = ip.To4(); ip == nil { return false }
    for _, it := range NETS_1918 {
        if ip.Mask(it.mask).Equal(it.ip) {
            return true
        }
    }
    return false
}

func (self *sipTransactionManager) rcache_put(checksum string, entry *sipTMRetransmitO) {
    self.rcache_lock.Lock()
    defer self.rcache_lock.Unlock()
    self.rcache_put_no_lock(checksum, entry)
}

func (self *sipTransactionManager) rcache_put_no_lock(checksum string, entry *sipTMRetransmitO) {
    self.l1rcache[checksum] = entry
}

func (self *sipTransactionManager) rcache_get_no_lock(checksum string) (entry *sipTMRetransmitO, ok bool) {
    entry, ok = self.l1rcache[checksum]
    if ok { return }
    entry, ok = self.l2rcache[checksum]
    return
}

func (self *sipTransactionManager) rcache_set_call_id(checksum, call_id string) {
    self.rcache_lock.Lock()
    defer self.rcache_lock.Unlock()
    if it, ok := self.rcache_get_no_lock(checksum); ok {
        it.call_id = call_id
    } else {
        self.rcache_put_no_lock(checksum, &sipTMRetransmitO{
                                userv : nil,
                                data  : nil,
                                address : nil,
                                call_id : call_id,
                            })
    }
}

func (self *sipTransactionManager) handleIncoming(data []byte, address *sippy_net.HostPort, server sippy_net.Transport, rtime *sippy_time.MonoTime) {
    if len(data) < 32 {
        //self.config.SipLogger().Write(rtime, retrans.call_id, "RECEIVED message from " + address.String() + ":\n" + string(data))
        //self.logError("The message is too short from " + address.String() + ":\n" + string(data))
        return
    }
    checksum := fmt.Sprintf("%x", md5.Sum(data))
    self.rcache_lock.Lock()
    retrans, ok := self.rcache_get_no_lock(checksum)
    if ok {
        self.rcache_lock.Unlock()
        self.config.SipLogger().Write(rtime, retrans.call_id, "RECEIVED message from " + address.String() + ":\n" + string(data))
        if retrans.data == nil {
            return
        }
        self.transmitData(retrans.userv, retrans.data, retrans.address, "", retrans.call_id, 0)
        return
    }
    self.rcache_put_no_lock(checksum, &sipTMRetransmitO{
                                userv : nil,
                                data  : nil,
                                address : nil,
                            })
    self.rcache_lock.Unlock()
    if string(data[:7]) == "SIP/2.0" {
        self.process_response(rtime, data, checksum, address, server)
    } else {
        self.process_request(rtime, data, checksum, address, server)
    }
}

func (self *sipTransactionManager) process_response(rtime *sippy_time.MonoTime, data []byte, checksum string, address *sippy_net.HostPort, server sippy_net.Transport) {
    var resp *sipResponse
    var err error
    var tid *sippy_header.TID
    var contact *sippy_header.SipAddress

    resp, err = ParseSipResponse(data, rtime, self.config)
    if err != nil {
        self.config.SipLogger().Write(rtime, "", "RECEIVED message from " + address.String() + ":\n" + string(data))
        self.logBadMessage("can't parse SIP response from " + address.String() + ":" + err.Error(), data)
        return
    }
    tid, err = resp.GetTId(true /*wCSM*/, true/*wBRN*/, false /*wTTG*/)
    if err != nil {
        self.config.SipLogger().Write(rtime, "", "RECEIVED message from " + address.String() + ":\n" + string(data))
        self.logBadMessage("can't parse SIP response from " + address.String() + ":" + err.Error(), data)
        return
    }
    self.config.SipLogger().Write(rtime, tid.CallId, "RECEIVED message from " + address.String() + ":\n" + string(data))

    if resp.scode < 100 || resp.scode > 999 {
        self.logBadMessage("invalid status code in SIP response" + address.String() + ":\n" + string(data), data)
        self.rcache_set_call_id(checksum, tid.CallId)
        return
    }
    self.tclient_lock.Lock()
    t, ok := self.tclient[*tid]
    self.tclient_lock.Unlock()
    if !ok {
        //print 'no transaction with tid of %s in progress' % str(tid)
        if len(resp.vias) > 1 {
            var via0 *sippy_header.SipViaBody

            via0, err = resp.vias[0].GetBody()
            if err != nil {
                self.logBadMessage(err.Error(), data)
                return
            }
            taddr := via0.GetTAddr(self.config)
            if taddr.Port.String() != self.config.SipPort().String() {
                if len(resp.contacts) == 0 {
                    self.logBadMessage("OnUdpPacket: no Contact: in SIP response", data)
                    return
                }
                if ! resp.contacts[0].Asterisk {
                    contact, err = resp.contacts[0].GetBody(self.config)
                    if err != nil {
                        self.logBadMessage(err.Error(), data)
                        return
                    }
                    curl := contact.GetUrl()
                    //print 'curl.host = %s, curl.port = %d,  address[1] = %d' % (curl.host, curl.port, address[1])
                    if check1918(curl.Host.String()) || curl.Port.String() != address.Port.String() {
                        curl.Host = sippy_net.NewMyAddress(taddr.Host.String())
                        curl.Port = sippy_net.NewMyPort(taddr.Port.String())
                    }
                }
                data = resp.Bytes()
                call_id := ""
                if resp.call_id != nil {
                    call_id = resp.call_id.CallId
                }
                self.transmitData(server, data, taddr, checksum, call_id, 0)
            }
        }
        self.rcache_set_call_id(checksum, tid.CallId)
        return
    }
    t.Lock()
    defer t.Unlock()
    if self.nat_traversal && len(resp.contacts) > 0 && !resp.contacts[0].Asterisk && ! check1918(t.GetHost()) {
        contact, err = resp.contacts[0].GetBody(self.config)
        if err != nil {
            self.logBadMessage(err.Error(), data)
            return
        }
        curl := contact.GetUrl()
        if check1918(curl.Host.String()) {
            host, port := address.Host.String(), address.Port.String()
            curl.Host, curl.Port = sippy_net.NewMyAddress(host), sippy_net.NewMyPort(port)
        }
    }
    host, port := address.Host.String(), address.Port.String()
    resp.source = sippy_net.NewHostPort(host, port)
    sippy_utils.SafeCall(func() { t.IncomingResponse(resp, checksum) }, nil, self.config.ErrorLogger())
}

func (self *sipTransactionManager) process_request(rtime *sippy_time.MonoTime, data []byte, checksum string, address *sippy_net.HostPort, server sippy_net.Transport) {
    var req *sipRequest
    var err error
    var tids []*sippy_header.TID
    var via0 *sippy_header.SipViaBody

    req, err = ParseSipRequest(data, rtime, self.config)
    if err != nil {
        switch errt := err.(type) {
        case *ESipParseException:
            if errt.sip_response != nil {
                self.transmitMsg(server, errt.sip_response, address, checksum, errt.sip_response.GetCallId().CallId)
            }
        }
        self.config.SipLogger().Write(rtime, "", "RECEIVED message from " + address.String() + ":\n" + string(data))
        self.logBadMessage("can't parse SIP request from " + address.String() + ": " + err.Error(), data)
        return
    }
    tids, err = req.getTIds()
    if err != nil {
        self.config.SipLogger().Write(rtime, "", "RECEIVED message from " + address.String() + ":\n" + string(data))
        self.logBadMessage(err.Error(), data)
        return
    }
    self.config.SipLogger().Write(rtime, tids[0].CallId, "RECEIVED message from " + address.String() + ":\n" + string(data))
    via0, err = req.vias[0].GetBody()
    if err != nil {
        self.logBadMessage(err.Error(), data)
        return
    }
    ahost, aport := via0.GetAddr(self.config)
    rhost, rport := address.Host.String(), address.Port.String()
    if self.nat_traversal && rport != aport && check1918(ahost) {
        req.nated = true
    }
    if ahost != rhost {
        via0.SetReceived(rhost)
    }
    if via0.HasRport() || req.nated {
        via0.SetRport(&rport)
    }
    if self.nat_traversal && len(req.contacts) > 0 && !req.contacts[0].Asterisk && len(req.vias) == 1 {
        var contact *sippy_header.SipAddress

        contact, err = req.contacts[0].GetBody(self.config)
        curl := contact.GetUrl()
        if check1918(curl.Host.String()) {
            tmp_host, tmp_port := address.Host.String(), address.Port.String()
            curl.Port = sippy_net.NewMyPort(tmp_port)
            curl.Host = sippy_net.NewMyAddress(tmp_host)
            req.nated = true
        }
    }
    host, port := address.Host.String(), address.Port.String()
    req.source = sippy_net.NewHostPort(host, port)
    self.incomingRequest(req, checksum, tids, server, data)
}

// 1. Client transaction methods
func (self *sipTransactionManager) CreateClientTransaction(req sippy_types.SipRequest, resp_receiver sippy_types.ResponseReceiver, session_lock sync.Locker, laddress *sippy_net.HostPort, userv sippy_net.Transport, req_out_cb func(sippy_types.SipRequest)) (sippy_types.ClientTransaction, error) {
    var tid *sippy_header.TID
    var err error
    var t *clientTransaction

    if self == nil {
        return nil, errors.New("BUG: Attempt to initiate transaction from terminated dialog!!!")
    }
    target := req.GetTarget()
    if userv == nil {
        var uv sippy_net.Transport
        if laddress != nil {
            uv = self.l4r.getServer(laddress, /*is_local =*/ true)
        }
        if uv == nil {
            uv = self.l4r.getServer(target, /*is_local =*/ false)
        }
        if uv != nil {
            userv = uv
        }
    }
    if userv == nil {
        return nil, errors.New("BUG: cannot get userv from local4remote!!!")
    }
    tid, err = req.GetTId(true /*wCSM*/, true/*wBRN*/, false /*wTTG*/)
    if err != nil {
        return nil, err
    }
    self.tclient_lock.Lock()
    if _, ok := self.tclient[*tid]; ok {
        self.tclient_lock.Unlock()
        return nil, errors.New("BUG: Attempt to initiate transaction with the same TID as existing one!!!")
    }
    data := []byte(req.LocalStr(userv.GetLAddress(), false /* compact */))
    t, err = NewClientTransactionObj(req, tid, userv, data, self, resp_receiver, session_lock, target, req_out_cb)
    if err != nil {
        return nil, err
    }
    self.tclient[*tid] = t
    self.tclient_lock.Unlock()
    return t, nil
}

func (self *sipTransactionManager) BeginClientTransaction(req sippy_types.SipRequest, tr sippy_types.ClientTransaction) {
    tr.StartTimers()
    tr.BeforeRequestSent(req)
    tr.TransmitData()
}

func (self *sipTransactionManager) BeginNewClientTransaction(req sippy_types.SipRequest, resp_receiver sippy_types.ResponseReceiver, session_lock sync.Locker, laddress *sippy_net.HostPort, userv sippy_net.Transport, req_out_cb func(sippy_types.SipRequest)) {
    tr, err := self.CreateClientTransaction(req, resp_receiver, session_lock, laddress, userv, req_out_cb)
    if err != nil {
        self.config.ErrorLogger().Error(err.Error())
    } else {
        self.BeginClientTransaction(req, tr)
    }
}

// 2. Server transaction methods
func (self *sipTransactionManager) incomingRequest(req *sipRequest, checksum string, tids []*sippy_header.TID, server sippy_net.Transport, data []byte) {
    var tid *sippy_header.TID
    var err error

    self.tclient_lock.Lock()
    for _, tid = range tids {
        if _, ok := self.tclient[*tid]; ok {
            var via0 *sippy_header.SipViaBody

            self.tclient_lock.Unlock()
            resp := req.GenResponse(482, "Loop Detected", /*body*/ nil, /*server*/ nil)
            via0, err = resp.GetVias()[0].GetBody()
            if err != nil {
                self.logBadMessage("cannot parse via: " + err.Error(), data)
            } else {
                hostport := via0.GetTAddr(self.config)
                self.transmitMsg(server, resp, hostport, checksum, tid.CallId)
            }
            return
        }
    }
    self.tclient_lock.Unlock()
    switch req.GetMethod() {
    case "ACK":
        tid, err = req.GetTId(false /*wCSM*/, false /*wBRN*/, true /*wTTG*/)
    case "PRACK":
        if rtid, err := req.GetRTId(); err == nil {
            self.rtid2tid_lock.Lock()
            tid = self.rtid2tid[*rtid]
            self.rtid2tid_lock.Unlock()
        }
    default:
        tid, err = req.GetTId(false /*wCSM*/, true /*wBRN*/, false /*wTTG*/)
    }
    if tid == nil {
        self.logBadMessage("cannot get transaction ID: ", data)
        return
    }
    if err != nil {
        self.logBadMessage("cannot get transaction ID: " + err.Error(), data)
        return
    }
    self.tserver_lock.Lock()
    t, ok := self.tserver[*tid]
    if ok {
        self.tserver_lock.Unlock()
        sippy_utils.SafeCall(func() { t.IncomingRequest(req, checksum) }, t, self.config.ErrorLogger())
        return
    }
    switch req.GetMethod() {
    case "ACK":
        self.tserver_lock.Unlock()
        // Some ACK that doesn't match any existing transaction.
        // Drop and forget it - upper layer is unlikely to be interested
        // to seeing this anyway.
        //println("unmatched ACK transaction - ignoring")
        self.rcache_set_call_id(checksum, tid.CallId)
    case "PRACK":
        // Some ACK that doesn't match any existing transaction.
        // Drop and forget it - upper layer is unlikely to be interested
        // to seeing this anyway.
        //print(datetime.now(), 'unmatched PRACK transaction - 481\'ing')
        //print(datetime.now(), 'rtid: %s, tid: %s, self.tserver: %s' % (str(rtid), str(tid), \
        //  str(self.tserver)))
        //sys.stdout.flush()
        via0, err := req.GetVias()[0].GetBody()
        if err != nil {
            self.logBadMessage("Cannot parse Via: " + err.Error(), data)
            return
        }
        resp := req.GenResponse(481, "Huh?", /*body*/ nil, /*server*/ nil)
        self.transmitMsg(server, resp, via0.GetTAddr(self.config), checksum, tid.CallId)
    case "CANCEL":
        var via0 *sippy_header.SipViaBody

        self.tserver_lock.Unlock()
        resp := req.GenResponse(481, "Call Leg/Transaction Does Not Exist", /*body*/ nil, /*server*/ nil)
        via0, err = resp.GetVias()[0].GetBody()
        if err != nil {
            self.logBadMessage("Cannot parse Via: " + err.Error(), data)
            return
        }
        self.transmitMsg(server, resp, via0.GetTAddr(self.config), checksum, tid.CallId)
    default:
        self.new_server_transaction(server, req, tid, checksum)
    }
}


func (self *sipTransactionManager) new_server_transaction(server sippy_net.Transport, req *sipRequest, tid *sippy_header.TID, checksum string) {
    var t sippy_types.ServerTransaction
    var err error

    /* Here the tserver_lock is already locked */
    var rval *sippy_types.Ua_context = nil
    //print 'new transaction', req.GetMethod()
    userv := server
    if server.GetLAddress().Host.String() == "0.0.0.0" || server.GetLAddress().Host.String() == "[::]" {
        // For messages received on the wildcard interface find
        // or create more specific server.
        userv = self.l4r.getServer(req.GetSource(), /*is_local*/ false)
        if userv == nil {
            self.logError("BUG! cannot create more specific server for transaction")
            userv = server
        }
    }
    t, err = NewServerTransaction(req, checksum, tid, userv, self)
    if err != nil {
        self.logError("cannot create server transaction: " + err.Error())
        return
    }
    t.Lock()
    defer t.Unlock()
    self.tserver[*tid] = t
    self.tserver_lock.Unlock()
    t.StartTimers()
    self.consumers_lock.Lock()
    consumers, ok := self.req_consumers[tid.CallId]
    var ua sippy_types.UA
    if ok {
        for _, c := range consumers {
            if c.IsYours(req, /*br0k3n_to =*/ false) {
                ua = c
                break
            }
        }
    }
    self.consumers_lock.Unlock()
    if ua != nil {
        t.UpgradeToSessionLock(ua.GetSessionLock())
        sippy_utils.SafeCall(func() { rval = ua.RecvRequest(req, t) }, nil, self.config.ErrorLogger())
    } else {
        if self.call_map == nil {
            self.rcache_put(checksum, &sipTMRetransmitO{
                                userv : nil,
                                data  : nil,
                                address : nil,
                            })
            return
        }
        var req_receiver sippy_types.RequestReceiver
        var resp sippy_types.SipResponse
        sippy_utils.SafeCall(func () { ua, req_receiver, resp = self.call_map.OnNewDialog(req, t) }, nil, self.config.ErrorLogger())
        if resp != nil {
            t.SendResponse(resp, false, nil)
            return
        } else {
            if ua != nil {
                t.UpgradeToSessionLock(ua.GetSessionLock())
            }
            if req_receiver != nil {
                rval = req_receiver.RecvRequest(req, t)
            }
            if ua != nil {
                self.consumers_lock.Lock()
                self.req_consumers[tid.CallId] = append(consumers, ua)
                self.consumers_lock.Unlock()
            }
        }
    }
    if rval == nil {
        if ! t.TimersAreActive() {
            self.tserver_del(tid)
            t.Cleanup()
        }
    } else {
        t.SetCancelCB(rval.CancelCB)
        t.SetNoackCB(rval.NoAckCB)
        if rval.Response != nil {
            t.SendResponse(rval.Response, false, nil)
        }
    }
}

func (self *sipTransactionManager) RegConsumer(consumer sippy_types.UA, call_id string) {
    self.consumers_lock.Lock()
    defer self.consumers_lock.Unlock()
    consumers, ok := self.req_consumers[call_id]
    if ! ok {
        consumers = make([]sippy_types.UA, 0)
    }
    consumers = append(consumers, consumer)
    self.req_consumers[call_id] = consumers
}

func (self *sipTransactionManager) UnregConsumer(consumer sippy_types.UA, call_id string) {
    // Usually there will be only one consumer per call_id, so that
    // optimize management for this case
    consumer.OnUnregister()
    self.consumers_lock.Lock()
    defer self.consumers_lock.Unlock()
    consumers, ok := self.req_consumers[call_id]
    if !ok {
        return
    }
    delete(self.req_consumers, call_id)
    if len(consumers) > 1 {
        for idx, c := range(consumers) {
            if c == consumer {
                consumers[idx] = nil
                consumers = append(consumers[:idx], consumers[idx + 1:]...)
                break
            }
        }
        self.req_consumers[call_id] = consumers
    }
}

func (self *sipTransactionManager) SendResponse(resp sippy_types.SipResponse, lock bool, ack_cb func(sippy_types.SipRequest)) {
    self.SendResponseWithLossEmul(resp, lock, ack_cb, 0)
}

func (self *sipTransactionManager) SendResponseWithLossEmul(resp sippy_types.SipResponse, lock bool, ack_cb func(sippy_types.SipRequest), lossemul int) {
    //print self.tserver
    tid, err := resp.GetTId(false /*wCSM*/, true /*wBRN*/, false /*wTTG*/)
    if err != nil {
        self.logError("Cannot get transaction ID for server transaction: " + err.Error())
        return
    }
    self.tserver_lock.Lock()
    t, ok := self.tserver[*tid]
    self.tserver_lock.Unlock()
    if ok {
        if lock {
            t.Lock()
            defer t.Unlock()
        }
        t.SendResponseWithLossEmul(resp, /*retrans*/ false, ack_cb, lossemul)
    } else {
        self.logError("Cannot get server transaction")
        return
    }
}

func (self *sipTransactionManager) transmitMsg(userv sippy_net.Transport, msg sippy_types.SipMsg, address *sippy_net.HostPort, cachesum string, call_id string) {
    data := msg.LocalStr(userv.GetLAddress(), false /*compact*/)
    self.transmitData(userv, []byte(data), address, cachesum, call_id, 0)
}

func (self *sipTransactionManager) transmitData(userv sippy_net.Transport, data []byte, address *sippy_net.HostPort, cachesum, call_id string, lossemul int /*=0*/) {
    self.transmitDataWithCb(userv, data, address, cachesum, call_id, lossemul, nil)
}

func (self *sipTransactionManager) transmitDataWithCb(userv sippy_net.Transport, data []byte, address *sippy_net.HostPort, cachesum, call_id string, lossemul int /*=0*/, on_complete func()) {
    logop := "SENDING"
    if lossemul == 0 {
        userv.SendToWithCb(data, address, on_complete)
    } else {
        logop = "DISCARDING"
    }
    self.config.SipLogger().Write(nil, call_id, logop + " message to " + address.String() + ":\n" + string(data))
    if len(cachesum) > 0 {
        if lossemul > 0 {
            lossemul--
        }
        self.rcache_put(cachesum, &sipTMRetransmitO{
            userv       : userv,
            data        : data,
            address     : address.GetCopy(),
            call_id     : call_id,
            lossemul    : lossemul,
        })
    }
}

func (self *sipTransactionManager) logError(msg string) {
    self.config.ErrorLogger().Error(msg)
}

func (self *sipTransactionManager) logBadMessage(msg string, data []byte) {
    self.config.ErrorLogger().Error(msg)
    arr := strings.Split(string(data), "\n")
    for _, l := range arr {
        self.config.ErrorLogger().Error(l)
    }
}

func (self *sipTransactionManager) tclient_del(tid *sippy_header.TID) {
    if tid == nil { return }
    self.tclient_lock.Lock()
    defer self.tclient_lock.Unlock()
    delete(self.tclient, *tid)
}

func (self *sipTransactionManager) tserver_del(tid *sippy_header.TID) {
    if tid == nil { return }
    self.tserver_lock.Lock()
    defer self.tserver_lock.Unlock()
    delete(self.tserver, *tid)
}

func (self *sipTransactionManager) tserver_replace(old_tid, new_tid *sippy_header.TID, t sippy_types.ServerTransaction) {
    self.tserver_lock.Lock()
    defer self.tserver_lock.Unlock()
    delete(self.tserver, *old_tid)
    self.tserver[*new_tid] = t
}

func (self *sipTransactionManager) Shutdown() {
    self.shutdown_chan <- 1
}

func (self *sipTransactionManager) beforeResponseSent(resp sippy_types.SipResponse) {
    if self.before_response_sent != nil {
        self.before_response_sent(resp)
    }
}

func (self *sipTransactionManager) SetBeforeResponseSent(cb func(sippy_types.SipResponse)) {
    self.before_response_sent = cb
}

func (self *sipTransactionManager) rtid_replace(ik *sippy_header.RTID, old_tid, new_tid *sippy_header.TID) {
    if saved_tid, ok := self.rtid2tid[*ik]; ok && *saved_tid == *old_tid {
        self.rtid2tid_lock.Lock()
        defer self.rtid2tid_lock.Unlock()
        self.rtid2tid[*ik] = new_tid
    }
}

func (self *sipTransactionManager) rtid_del(key *sippy_header.RTID) {
    self.rtid2tid_lock.Lock()
    defer self.rtid2tid_lock.Unlock()
    delete(self.rtid2tid, *key)
}

func (self *sipTransactionManager) rtid_put(key *sippy_header.RTID, value *sippy_header.TID) {
    self.rtid2tid_lock.Lock()
    defer self.rtid2tid_lock.Unlock()
    self.rtid2tid[*key] = value
}
