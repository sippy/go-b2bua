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
package sippy_types

import (
    "sync"
    "time"

    "sippy/conf"
    "sippy/time"
    "sippy/headers"
)

type CallController interface {
    RecvEvent(CCEvent, UA)
}

type RequestReceiver interface {
    RecvRequest(SipRequest, ServerTransaction) *Ua_context
}

type ResponseReceiver interface {
    RecvResponse(SipResponse, ClientTransaction)
}

type CallMap interface {
    OnNewDialog(SipRequest) (UA, RequestReceiver, SipResponse)
}

type SipMsg interface {
    GetSipUserAgent() *sippy_header.SipUserAgent
    GetSipServer() *sippy_header.SipServer
    LocalStr(hostport *sippy_conf.HostPort, compact bool) string
    GetCSeq() *sippy_header.SipCSeq
    GetTId(wCSM, wBRN, wTTG bool) *sippy_header.TID
    GetTo() *sippy_header.SipTo
    GetReason() *sippy_header.SipReason
    AppendHeader(hdr sippy_header.SipHeader)
    GetVias() []*sippy_header.SipVia
    GetCallId() *sippy_header.SipCallId
    SetRtime(*sippy_time.MonoTime)
    GetTarget() *sippy_conf.HostPort
    SetTarget(address *sippy_conf.HostPort)
    InsertFirstVia(*sippy_header.SipVia)
    RemoveFirstVia()
    SetRoutes([]*sippy_header.SipRoute)
    GetFrom() *sippy_header.SipFrom
    GetRtime() *sippy_time.MonoTime
    GetAlso() []*sippy_header.SipAlso
    GetBody() MsgBody
    SetBody(MsgBody)
    GetContacts() []*sippy_header.SipContact
    GetRecordRoutes() []*sippy_header.SipRecordRoute
    GetCGUID() *sippy_header.SipCiscoGUID
    GetH323ConfId() *sippy_header.SipH323ConfId
    GetSipAuthorization() *sippy_header.SipAuthorization
    GetSource() *sippy_conf.HostPort
    GetFirstHF(string) sippy_header.SipHeader
    GetHFs(string) []sippy_header.SipHeader
}

type SipRequest interface {
    SipMsg
    GetSipProxyAuthorization() *sippy_header.SipProxyAuthorization
    GenResponse(int, string, MsgBody, *sippy_header.SipServer) SipResponse
    GetMethod() string
    GetExpires() *sippy_header.SipExpires
    GenACK(to *sippy_header.SipTo, config sippy_conf.Config) SipRequest
    GenCANCEL(sippy_conf.Config) SipRequest
    GetRURI() *sippy_header.SipURL
    SetRURI(ruri *sippy_header.SipURL)
    GetReferTo() *sippy_header.SipReferTo
    GetNated() bool
}

type SipResponse interface {
    SipMsg
    GetSCode() (int, string)
    SetSCode(int, string)
    GetSCodeNum() int
    GetSipWWWAuthenticate() *sippy_header.SipWWWAuthenticate
    GetSipProxyAuthenticate() *sippy_header.SipProxyAuthenticate
    SetReason(string)
    GetCopy() SipResponse
}

type UdpServer interface {
    GetLaddress() *sippy_conf.HostPort
    SendTo([]byte, string, string)
}

type MsgBody interface {
    String() string
    GetMtype() string
    LocalStr(hostport *sippy_conf.HostPort) string
    GetCopy() MsgBody
    NeedsUpdate() bool
    GetParsedBody() ParsedMsgBody
}

type ParsedMsgBody interface {
    String() string
    LocalStr(hostport *sippy_conf.HostPort) string
    GetCopy() ParsedMsgBody
    SetCHeaderAddr(string)
}

type UA interface {
    RequestReceiver
    ResponseReceiver
    GetSessionLock() sync.Locker
    RecvEvent(CCEvent)
    SipTM() SipTransactionManager
    GetSetupTs() *sippy_time.MonoTime
    SetSetupTs(*sippy_time.MonoTime)
    SetDisconnectTs(*sippy_time.MonoTime)
    GetOrigin() string
    SetOrigin(string)
    HasOnLocalSdpChange() bool
    OnLocalSdpChange(MsgBody, CCEvent, func())
    ResetOnLocalSdpChange()
    OnRemoteSdpChange(MsgBody, SipMsg, func(MsgBody))
    HasOnRemoteSdpChange() bool
    ResetOnRemoteSdpChange()
    SetCallId(*sippy_header.SipCallId)
    GetCallId() *sippy_header.SipCallId
    SetRTarget(*sippy_header.SipURL)
    GetRAddr() *sippy_conf.HostPort
    SetRAddr(addr *sippy_conf.HostPort)
    GetRAddr0() *sippy_conf.HostPort
    SetRAddr0(addr *sippy_conf.HostPort)
    GetRTarget() *sippy_header.SipURL
    SetRUri(*sippy_header.SipTo)
    GetRuriUserparams() []string
    GetRUri() *sippy_header.SipTo
    GetToUsername() string
    GetUsername() string
    GetPassword() string
    SetLUri(*sippy_header.SipFrom)
    GetLUri() *sippy_header.SipFrom
    GetFromDomain() string
    GetLTag() string
    SetLCSeq(int)
    SetLContact(*sippy_header.SipContact)
    GetLContact() *sippy_header.SipContact
    SetRoutes([]*sippy_header.SipRoute)
    GetCGUID() *sippy_header.SipCiscoGUID
    SetCGUID(*sippy_header.SipCiscoGUID)
    SetH323ConfId(*sippy_header.SipH323ConfId)
    GetLSDP() MsgBody
    SetLSDP(MsgBody)
    GetRSDP() MsgBody
    SetRSDP(MsgBody)
    GenRequest(method string, body MsgBody, nonce string, realm string, SipXXXAuthorization sippy_header.NewSipXXXAuthorizationFunc, extra_headers ...sippy_header.SipHeader) SipRequest
    IncLCSeq()
    GetSourceAddress() *sippy_conf.HostPort
    SetSourceAddress(*sippy_conf.HostPort)
    GetClientTransaction() ClientTransaction
    SetClientTransaction(ClientTransaction)
    GetOutboundProxy() *sippy_conf.HostPort
    GetNoReplyTime() time.Duration
    GetExpireTime() time.Duration
    SetExpireTime(time.Duration)
    GetNoProgressTime() time.Duration
    StartNoReplyTimer(*sippy_time.MonoTime)
    StartNoProgressTimer(*sippy_time.MonoTime)
    StartExpireTimer(*sippy_time.MonoTime)
    CancelExpireTimer()
    GetDiscCbs() []OnDisconnectListener
    GetFailCbs() []OnFailureListener
    GetConnCbs() []OnConnectListener
    GetRingCbs() []OnRingingListener
    IsYours(SipRequest, bool) bool
    GetLocalUA() *sippy_header.SipUserAgent
    SetLocalUA(*sippy_header.SipUserAgent)
    Enqueue(CCEvent)
    GetUasResp() SipResponse
    SetUasResp(SipResponse)
    CancelCreditTimer()
    StartCreditTimer(*sippy_time.MonoTime)
    SetCreditTime(time.Duration)
    ShouldUseRefer() bool
    GetState() UaState
    Disconnect(*sippy_time.MonoTime)
    SetKaInterval(time.Duration)
    GetKaInterval() time.Duration
    OnDead()
    GetGoDeadTimeout() time.Duration
    ChangeState(UaState)
    GetLastScode() int
    SetLastScode(int)
    HasNoReplyTimer() bool
    CancelNoReplyTimer()
    GetNpMtime() *sippy_time.MonoTime
    SetNpMtime(*sippy_time.MonoTime)
    GetExMtime() *sippy_time.MonoTime
    SetExMtime(*sippy_time.MonoTime)
    SetP100Ts(*sippy_time.MonoTime)
    HasNoProgressTimer() bool
    CancelNoProgressTimer()
    DelayedRemoteSdpUpdate(event CCEvent, remote_sdp_body MsgBody)
    GetP1xxTs() *sippy_time.MonoTime
    SetP1xxTs(*sippy_time.MonoTime)
    UpdateRouting(SipResponse, bool, bool)
    SetConnectTs(*sippy_time.MonoTime)
    SetBranch(string)
    SetAuth(sippy_header.SipHeader)
    GetNrMtime() *sippy_time.MonoTime
    SetNrMtime(*sippy_time.MonoTime)
    SendUasResponse(t ServerTransaction, scode int, reason string, body MsgBody /*= nil*/, contact *sippy_header.SipContact /*= nil*/, ack_wait bool /*false*/, extra_headers ...sippy_header.SipHeader)
    EmitEvent(CCEvent)
    String() string
    GetPendingTr() ClientTransaction
    SetPendingTr(ClientTransaction)
    GetLateMedia() bool
    SetLateMedia(bool)
    GetPassAuth() bool
}

type baseTransaction interface {
    GetHost() string
    Lock()
    Unlock()
    StartTimers()
}

type ClientTransaction interface {
    baseTransaction
    IncomingResponse(resp SipResponse, checksum string)
    SetOutboundProxy(*sippy_conf.HostPort)
    Cancel(...sippy_header.SipHeader)
    GetACK() SipRequest
    SendACK()
    SetUAck(bool)
}

type ServerTransaction interface {
    baseTransaction
    IncomingRequest(req SipRequest, checksum string)
    TimersAreActive() bool
    SetCancelCB(func(*sippy_time.MonoTime, SipRequest))
    SetNoackCB(func(*sippy_time.MonoTime))
    SendResponse(resp SipResponse, retrans bool, ack_cb func(SipRequest))
    Cleanup()
    UpgradeToSessionLock(sync.Locker)
    SetServer(*sippy_header.SipServer)
}

type SipTransactionManager interface {
    RegConsumer(UA, string)
    UnregConsumer(UA, string)
    NewClientTransaction(SipRequest, ResponseReceiver, sync.Locker, *sippy_conf.HostPort, UdpServer) (ClientTransaction, error)
    SendResponse(resp SipResponse, lock bool, ack_cb func(SipRequest))
    Run()
}

type UaState interface {
    RecvEvent(CCEvent) (UaState, error)
    RecvResponse(SipResponse, ClientTransaction) UaState
    RecvRequest(SipRequest, ServerTransaction) UaState
    Cancel(*sippy_time.MonoTime, SipRequest)
    OnStateChange()
    String() string
    OnActivation()
    RecvACK(SipRequest)
    IsConnected() bool
}

type CCEvent interface {
    GetSeq() int64
    GetRtime() *sippy_time.MonoTime
    GetOrigin() string
    GetExtraHeaders() []sippy_header.SipHeader
    SetReason(*sippy_header.SipReason)
    GetReason() *sippy_header.SipReason
    String() string
}

type StatefulProxy interface {
    RequestReceiver
}

type OnRingingListener interface {
    OnRinging(*sippy_time.MonoTime, string, int)
}

type OnDisconnectListener interface {
    OnDisconnect(*sippy_time.MonoTime, string, int)
}

type OnFailureListener interface {
    OnFailure(*sippy_time.MonoTime, string, int)
}

type OnConnectListener interface {
    OnConnect(*sippy_time.MonoTime, string)
}

type OnDeadListener interface {
    OnDead()
}
