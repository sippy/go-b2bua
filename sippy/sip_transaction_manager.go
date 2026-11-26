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
    "encoding/hex"
    "errors"
    "net"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"
    "time"
    "unicode"
    "unicode/utf8"

    "github.com/sippy/go-b2bua/sippy/conf"
    "github.com/sippy/go-b2bua/sippy/headers"
    "github.com/sippy/go-b2bua/sippy/net"
    "github.com/sippy/go-b2bua/sippy/types"
    "github.com/sippy/go-b2bua/sippy/time"
    "github.com/sippy/go-b2bua/sippy/utils"
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
    before_response_sent func(sippy_types.SipResponse)
    rtid2tid        map[sippy_header.RTID]*sippy_header.TID
    rtid2tid_lock   sync.Mutex
    loss_emul       atomic.Pointer[sipLossBudget]
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
        rtid2tid        : make(map[sippy_header.RTID]*sippy_header.TID),
    }
    // Lock the lock here, otherwise we might get request in too
    // early for us to start processing it
    self.rcache_lock.Lock()
    defer self.rcache_lock.Unlock()
    self.l4r, err = NewLocal4Remote(config, self.handleIncoming)
    if err != nil {
        return nil, err
    }
    go func() {
        for {
            select {
            case <-self.shutdown_chan:
                return
            case <-time.After(32 * time.Second):
                self.rCachePurge()
            }
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

const invalidSuffix = ".invalid"

func check7118(host string) bool {
    return strings.HasSuffix(host, invalidSuffix)
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

// sipLogText is a display-only view. SIP framing, transport, and the
// retransmission cache continue to use the original octets.
func sipLogText(data []byte) string {
    const digits = "0123456789abcdef"
    var text strings.Builder
    start := 0
    for i := 0; i < len(data); {
        b := data[i]
        if (b >= ' ' && b <= '~') || b == '\r' || b == '\n' || b == '\t' {
            i++
            continue
        }
        r, size := rune(b), 1
        if b >= utf8.RuneSelf {
            r, size = utf8.DecodeRune(data[i:])
            if unicode.IsPrint(r) && !(r == utf8.RuneError && size == 1) {
                i += size
                continue
            }
        }
        if start == 0 {
            text.Grow(len(data))
        }
        text.Write(data[start:i])
        if r == utf8.RuneError && size == 1 {
            text.WriteString(`\x`)
            text.WriteByte(digits[b>>4])
            text.WriteByte(digits[b&15])
        } else {
            quoted := strconv.QuoteRune(r)
            text.WriteString(quoted[1 : len(quoted)-1])
        }
        i += size
        start = i
    }
    if start == 0 {
        return string(data)
    }
    text.Write(data[start:])
    return text.String()
}

type sipLogDirection bool

const (
    sipLogIncoming sipLogDirection = false
    sipLogOutgoing sipLogDirection = true
)

func (self *sipTransactionManager) logMsg(rtime *sippy_time.MonoTime, call_id string,
  direction sipLogDirection, discarded bool, address *sippy_net.HostPort, data []byte) {
    action, peer := "RECEIVED", " from "
    if direction == sipLogOutgoing {
        action, peer = "SENDING", " to "
    }
    if discarded {
        action = "DISCARDING"
    }
    msg := action + " message" + peer + address.String() + ":\n" + sipLogText(data) + "\n"
    self.config.SipLogger().Write(rtime, call_id, msg)
}

func (self *sipTransactionManager) handleIncoming(data []byte, address *sippy_net.HostPort, server sippy_net.Transport, rtime *sippy_time.MonoTime) {
    if len(data) < 32 {
        return
    }
    if budget := self.loss_emul.Load(); budget != nil && consumeSipLoss(&budget.recv) {
        self.logMsg(rtime, "", sipLogIncoming, true, address, data)
        return
    }
    sum := md5.Sum(data)
    checksum := hex.EncodeToString(sum[:])
    self.rcache_lock.Lock()
    retrans, ok := self.rcache_get_no_lock(checksum)
    if ok {
        // The call ID can be filled in by the first receive path.
        cached := *retrans
        self.rcache_lock.Unlock()
        retrans = &cached
        self.logMsg(rtime, retrans.call_id, sipLogIncoming, false, address, data)
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
        self.logMsg(rtime, "", sipLogIncoming, false, address, data)
        self.logBadMessage("can't parse SIP response from " + address.String() + ":" + err.Error(), data)
        return
    }
    tid, err = resp.GetTId(true /*wCSM*/, true/*wBRN*/, false /*wTTG*/)
    if err != nil {
        self.logMsg(rtime, "", sipLogIncoming, false, address, data)
        self.logBadMessage("can't parse SIP response from " + address.String() + ":" + err.Error(), data)
        return
    }
    self.logMsg(rtime, tid.CallId, sipLogIncoming, false, address, data)

    if resp.scode < 100 || resp.scode > 999 {
        self.logBadMessage("invalid status code in SIP response from " + address.String(), data)
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
    // RFC 7118: for SIP-over-WebSocket the response must be sent back on
    // the same open connection; rewrite any Contact host to the connection
    // identifier so later requests in the dialog target it.
    rreceived := ""
    if address.Received != "" && check7118(address.Received) {
        rreceived = address.Received
        if len(resp.contacts) > 0 && !resp.contacts[0].Asterisk {
            if cbody, err := resp.contacts[0].GetBody(self.config); err == nil {
                cbody.GetUrl().Host = sippy_net.NewMyAddress(rreceived)
            }
        }
    }
    resp.source = sippy_net.NewHostPortWithReceived(host, port, rreceived)
    sippy_utils.SafeCall(func() { t.IncomingResponse(resp, checksum) }, nil, self.config.ErrorLogger())
}

func (self *sipTransactionManager) process_request(rtime *sippy_time.MonoTime, data []byte, checksum string, address *sippy_net.HostPort, server sippy_net.Transport) {
    var err error
    var tids []*sippy_header.TID
    var via0 *sippy_header.SipViaBody

    req, perr := parseSipRequest(data, rtime, self.config)
    if perr != nil {
        self.logMsg(rtime, "", sipLogIncoming, false, address, data)
        self.logBadMessage("can't parse SIP request from " + address.String() + ": " + perr.Error(), data)
        // Reply only if the transaction headers can be parsed and agree
        // with the request line. ACK never gets a response.
        if req == nil {
            return
        }
        method := req.GetMethod()
        if strings.EqualFold(method, "ACK") {
            return
        }
        tid, err := req.GetTId(true, true, true)
        if err != nil || tid.CallId == "" || tid.CSeqMethod != method {
            return
        }
        var response sippy_types.SipResponse
        if req.ruri != nil {
            response = perr.GetResponse(req)
        } else {
            response = req.GenResponse(400, "Bad Request", nil, nil)
        }
        self.transmitMsg(server, response, address, checksum, tid.CallId)
        return
    }
    tids, err = req.getTIds()
    if err != nil {
        self.logMsg(rtime, "", sipLogIncoming, false, address, data)
        self.logBadMessage(err.Error(), data)
        return
    }
    self.logMsg(rtime, tids[0].CallId, sipLogIncoming, false, address, data)
    via0, err = req.vias[0].GetBody()
    if err != nil {
        self.logBadMessage(err.Error(), data)
        return
    }
    ahost, aport := via0.GetAddr(self.config)
    rhost, rport := address.Host.String(), address.Port.String()
    // RFC 7118: for SIP-over-WebSocket the received parameter carries the
    // connection identifier (not the client IP) so that every response of
    // the dialog is routed back to exactly the open connection it came
    // from, even when several connections share the same client IP.
    rreceived := ""
    if address.Received != "" && check7118(address.Received) {
        rreceived = address.Received
    }
    if self.nat_traversal && rport != aport && (check1918(ahost) || check7118(ahost)) {
        req.nated = true
    }
    if ahost != rhost {
        if rreceived != "" {
            via0.SetReceived(rreceived)
        } else {
            via0.SetReceived(rhost)
        }
    }
    if via0.HasRport() || req.nated {
        via0.SetRport(&rport)
    }
    if self.nat_traversal && len(req.contacts) > 0 && !req.contacts[0].Asterisk && len(req.vias) == 1 {
        var contact *sippy_header.SipAddress

        contact, err = req.contacts[0].GetBody(self.config)
        if err != nil {
            self.logBadMessage("Bad Contact: " + err.Error(), data)
            return
        }
        curl := contact.GetUrl()
        if check1918(curl.Host.String()) {
            tmp_host, tmp_port := address.Host.String(), address.Port.String()
            curl.Port = sippy_net.NewMyPort(tmp_port)
            curl.Host = sippy_net.NewMyAddress(tmp_host)
            req.nated = true
        }
    }
    // RFC 7118: rewrite the Contact host to the connection identifier so
    // that all subsequent requests of the dialog (ACK, BYE, re-INVITE)
    // are routed to the same open WebSocket connection.
    if rreceived != "" && len(req.contacts) > 0 && len(req.vias) == 1 && !req.contacts[0].Asterisk {
        if cbody, err := req.contacts[0].GetBody(self.config); err == nil {
            cbody.GetUrl().Host = sippy_net.NewMyAddress(rreceived)
        }
    }
    host, port := address.Host.String(), address.Port.String()
    req.source = sippy_net.NewHostPortWithReceived(host, port, rreceived)
    self.incomingRequest(req, checksum, tids, server, data)
}

// 1. Client transaction methods
func (self *sipTransactionManager) CreateClientTransaction(req sippy_types.SipRequest,
        resp_receiver sippy_types.ResponseReceiver, session_lock sync.Locker,
        laddress *sippy_net.HostPort, userv sippy_net.Transport,
        eh []sippy_header.SipHeader, req_out_cb func(sippy_types.SipRequest)) (sippy_types.ClientTransaction, error) {
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
    // RFC 7118: server-generated requests sent over a WebSocket connection
    // must carry SIP/2.0/WSS (TLS) or SIP/2.0/WS (plaintext) in the top
    // Via instead of the default UDP.
    if ws, is_wss := userv.(sippy_net.WebSocketTransport); is_wss && len(req.GetVias()) > 0 {
        if via0, verr := req.GetVias()[0].GetBody(); verr == nil {
            via0.SetSipVer("SIP/2.0/" + ws.WssProtocol())
        }
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
    t, err = NewClientTransactionObj(req, tid, userv, data, self, resp_receiver, session_lock, target, eh, req_out_cb)
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
    tr, err := self.CreateClientTransaction(req, resp_receiver, session_lock, laddress, userv, nil, req_out_cb)
    if err != nil {
        self.config.ErrorLogger().Error(err.Error())
    } else {
        self.BeginClientTransaction(req, tr)
    }
}

// 2. Server transaction methods
func (self *sipTransactionManager) incomingRequest(req *sipRequest, checksum string, tids []*sippy_header.TID, server sippy_net.Transport, data []byte) {
    var tid *sippy_header.TID
    var rtid *sippy_header.RTID
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
        if rtid, err = req.GetRTId(); err != nil {
            self.logBadMessage("cannot get transaction ID: " + err.Error(), data)
            return
        }
        self.rtid2tid_lock.Lock()
        tid = self.rtid2tid[*rtid]
        self.rtid2tid_lock.Unlock()
    default:
        tid, err = req.GetTId(false /*wCSM*/, true /*wBRN*/, false /*wTTG*/)
        if err != nil {
            self.logBadMessage("cannot get transaction ID: " + err.Error(), data)
            return
        }
    }
    if tid == nil {
        self.logBadMessage("cannot get transaction ID: ", data)
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
    if _, is_wss := server.(sippy_net.WebSocketTransport); !is_wss {
        if server.GetLAddress().Host.String() == "0.0.0.0" || server.GetLAddress().Host.String() == "[::]" {
            // For messages received on the wildcard interface find
            // or create more specific server. (Not applicable to SIP over
            // WebSocket: RFC 7118 requires every response of the dialog to
            // go back on the same open connection.)
            userv = self.l4r.getServer(req.GetSource(), /*is_local*/ false)
            if userv == nil {
                self.logError("BUG! cannot create more specific server for transaction")
                userv = server
            }
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
    discarded := lossemul != 0
    if !discarded {
        budget := self.loss_emul.Load()
        discarded = budget != nil && consumeSipLoss(&budget.send)
    }
    if !discarded {
        userv.SendToWithCb(data, address, on_complete)
    }
    self.logMsg(nil, call_id, sipLogOutgoing, discarded, address, data)
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

// Bound each untrusted field before escaping it, including parser errors that
// may quote input. This limit applies to error logs, not to SIP processing.
const sipBadLogFieldLimit = 4096

func (self *sipTransactionManager) logBadMessage(msg string, data []byte) {
    const truncated = " … [truncated]"
    var msgSuffix, dataSuffix string
    if len(msg) > sipBadLogFieldLimit {
        msg = msg[:sipBadLogFieldLimit]
        msgSuffix = truncated
    }
    if len(data) > sipBadLogFieldLimit {
        data = data[:sipBadLogFieldLimit]
        dataSuffix = truncated
    }
    self.config.ErrorLogger().Error(sipLogText([]byte(msg)) + msgSuffix + "\n" + sipLogText(data) + dataSuffix)
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
    close(self.shutdown_chan)
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

func (self *sipTransactionManager) SetNatTraversal(nat_traversal bool) {
    self.nat_traversal = nat_traversal
}
