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

    "sippy/headers"
    "sippy/net"
    "sippy/sdp"
    "sippy/time"
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
    OnNewDialog(SipRequest, ServerTransaction) (UA, RequestReceiver, SipResponse)
}

type SipMsg interface {
    GetSipUserAgent() *sippy_header.SipUserAgent
    GetSipServer() *sippy_header.SipServer
    LocalStr(hostport *sippy_net.HostPort, compact bool) string
    GetCSeq() *sippy_header.SipCSeq
    GetRSeq() *sippy_header.SipRSeq
    GetSipRAck() *sippy_header.SipRAck
    GetTId(wCSM, wBRN, wTTG bool) (*sippy_header.TID, error)
    GetTo() *sippy_header.SipTo
    GetReason() *sippy_header.SipReason
    AppendHeader(hdr sippy_header.SipHeader)
    GetVias() []*sippy_header.SipVia
    GetCallId() *sippy_header.SipCallId
    SetRtime(*sippy_time.MonoTime)
    GetTarget() *sippy_net.HostPort
    SetTarget(address *sippy_net.HostPort)
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
    GetSource() *sippy_net.HostPort
    GetFirstHF(string) sippy_header.SipHeader
    GetHFs(string) []sippy_header.SipHeader
    GetSL() string
    GetMaxForwards() *sippy_header.SipMaxForwards
    SetMaxForwards(*sippy_header.SipMaxForwards)
    GetRTId() (*sippy_header.RTID, error)
    GetSipRequire() []*sippy_header.SipRequire
    GetSipSupported() []*sippy_header.SipSupported
}

type SipRequest interface {
    SipMsg
    GetSipProxyAuthorization() *sippy_header.SipProxyAuthorization
    GenResponse(int, string, MsgBody, *sippy_header.SipServer) SipResponse
    GetMethod() string
    GetExpires() *sippy_header.SipExpires
    GenACK(to *sippy_header.SipTo) (SipRequest, error)
    GenCANCEL() (SipRequest, error)
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
    GetSCodeReason() string
    GetSipWWWAuthenticate() *sippy_header.SipWWWAuthenticate
    GetSipProxyAuthenticate() *sippy_header.SipProxyAuthenticate
    SetSCodeReason(string)
    GetCopy() SipResponse
}

type MsgBody interface {
    String() string
    GetMtype() string
    LocalStr(hostport *sippy_net.HostPort) string
    GetCopy() MsgBody
    NeedsUpdate() bool
    SetNeedsUpdate(bool)
    GetParsedBody() (ParsedMsgBody, error)
    AppendAHeader(string)
}

type ParsedMsgBody interface {
    String() string
    LocalStr(hostport *sippy_net.HostPort) string
    GetCopy() ParsedMsgBody
    SetCHeaderAddr(string)
    GetCHeader() *sippy_sdp.SdpConnecton
    GetSections() []*sippy_sdp.SdpMediaDescription
    SetSections([]*sippy_sdp.SdpMediaDescription)
    RemoveSection(int)
    GetOHeader() *sippy_sdp.SdpOrigin
    SetOHeader(*sippy_sdp.SdpOrigin)
    AppendAHeader(string)
}

type UA interface {
    OnUnregister()
    RequestReceiver
    ResponseReceiver
    GetSessionLock() sync.Locker
    RecvEvent(CCEvent)
    RecvACK(SipRequest)
    SipTM() SipTransactionManager
    GetSetupTs() *sippy_time.MonoTime
    SetSetupTs(*sippy_time.MonoTime)
    GetDisconnectTs() *sippy_time.MonoTime
    SetDisconnectTs(*sippy_time.MonoTime)
    GetOrigin() string
    SetOrigin(string)
    HasOnLocalSdpChange() bool
    OnLocalSdpChange(MsgBody, func(MsgBody)) error
    SetOnLocalSdpChange(OnLocalSdpChange)
    ResetOnLocalSdpChange()
    OnRemoteSdpChange(MsgBody, func(MsgBody)) error
    HasOnRemoteSdpChange() bool
    ResetOnRemoteSdpChange()
    SetCallId(*sippy_header.SipCallId)
    GetCallId() *sippy_header.SipCallId
    SetRTarget(*sippy_header.SipURL)
    GetRAddr() *sippy_net.HostPort
    SetRAddr(addr *sippy_net.HostPort)
    GetRAddr0() *sippy_net.HostPort
    SetRAddr0(addr *sippy_net.HostPort)
    GetRTarget() *sippy_header.SipURL
    SetRUri(*sippy_header.SipTo)
    GetRUri() *sippy_header.SipTo
    GetUsername() string
    SetUsername(string)
    GetPassword() string
    SetPassword(string)
    SetLUri(*sippy_header.SipFrom)
    GetLUri() *sippy_header.SipFrom
    GetLTag() string
    SetLCSeq(int)
    SetLContact(*sippy_header.SipContact)
    GetLContact() *sippy_header.SipContact
    GetLContacts() []*sippy_header.SipContact
    SetRoutes([]*sippy_header.SipRoute)
    GetCGUID() *sippy_header.SipCiscoGUID
    SetCGUID(*sippy_header.SipCiscoGUID)
    SetH323ConfId(*sippy_header.SipH323ConfId)
    GetLSDP() MsgBody
    SetLSDP(MsgBody)
    GetRSDP() MsgBody
    SetRSDP(MsgBody)
    GenRequest(method string, body MsgBody, nonce string, realm string, SipXXXAuthorization sippy_header.NewSipXXXAuthorizationFunc, extra_headers ...sippy_header.SipHeader) (SipRequest, error)
    IncLCSeq()
    GetSourceAddress() *sippy_net.HostPort
    SetSourceAddress(*sippy_net.HostPort)
    GetClientTransaction() ClientTransaction
    SetClientTransaction(ClientTransaction)
    GetOutboundProxy() *sippy_net.HostPort
    SetOutboundProxy(*sippy_net.HostPort)
    GetNoReplyTime() time.Duration
    SetNoReplyTime(time.Duration)
    GetExpireTime() time.Duration
    SetExpireTime(time.Duration)
    GetNoProgressTime() time.Duration
    SetNoProgressTime(time.Duration)
    StartNoReplyTimer(*sippy_time.MonoTime)
    StartNoProgressTimer(*sippy_time.MonoTime)
    StartExpireTimer(*sippy_time.MonoTime)
    CancelExpireTimer()
    DiscCb(*sippy_time.MonoTime, string, int, SipRequest)
    GetDiscCb() OnDisconnectListener
    SetDiscCb(OnDisconnectListener)
    FailCb(*sippy_time.MonoTime, string, int)
    GetFailCb() OnFailureListener
    SetFailCb(OnFailureListener)
    ConnCb(*sippy_time.MonoTime, string)
    GetConnCb() OnConnectListener
    SetConnCb(OnConnectListener)
    RingCb(*sippy_time.MonoTime, string, int)
    GetDeadCb() OnDeadListener
    SetDeadCb(OnDeadListener)
    IsYours(SipRequest, bool) bool
    GetLocalUA() *sippy_header.SipUserAgent
    SetLocalUA(*sippy_header.SipUserAgent)
    Enqueue(CCEvent)
    GetUasResp() SipResponse
    SetUasResp(SipResponse)
    CancelCreditTimer()
    StartCreditTimer(*sippy_time.MonoTime)
    SetCreditTime(time.Duration)
    ResetCreditTime(*sippy_time.MonoTime, map[int64]*sippy_time.MonoTime)
    ShouldUseRefer() bool
    GetState() UaState
    Disconnect(*sippy_time.MonoTime, string)
    SetKaInterval(time.Duration)
    GetKaInterval() time.Duration
    OnDead()
    OnUacSetupComplete()
    GetGoDeadTimeout() time.Duration
    ChangeState(UaState, func())
    GetLastScode() int
    SetLastScode(int)
    HasNoReplyTimer() bool
    CancelNoReplyTimer()
    GetNpMtime() *sippy_time.MonoTime
    SetNpMtime(*sippy_time.MonoTime)
    GetExMtime() *sippy_time.MonoTime
    SetExMtime(*sippy_time.MonoTime)
    GetP100Ts() *sippy_time.MonoTime
    SetP100Ts(*sippy_time.MonoTime)
    HasNoProgressTimer() bool
    CancelNoProgressTimer()
    DelayedRemoteSdpUpdate(event CCEvent, remote_sdp_body MsgBody)
    GetP1xxTs() *sippy_time.MonoTime
    SetP1xxTs(*sippy_time.MonoTime)
    UpdateRouting(SipResponse, bool, bool)
    GetConnectTs() *sippy_time.MonoTime
    SetConnectTs(*sippy_time.MonoTime)
    SetBranch(string)
    SetAuth(sippy_header.SipHeader)
    GetNrMtime() *sippy_time.MonoTime
    SetNrMtime(*sippy_time.MonoTime)
    SendUasResponse(t ServerTransaction, scode int, reason string, body MsgBody /*= nil*/, contacts []*sippy_header.SipContact /*= nil*/, ack_wait bool /*false*/, extra_headers ...sippy_header.SipHeader)
    EmitEvent(CCEvent)
    String() string
    GetPendingTr() ClientTransaction
    SetPendingTr(ClientTransaction)
    GetLateMedia() bool
    SetLateMedia(bool)
    GetPassAuth() bool
    GetOnLocalSdpChange() OnLocalSdpChange
    GetOnRemoteSdpChange() OnRemoteSdpChange
    SetOnRemoteSdpChange(OnRemoteSdpChange)
    GetRemoteUA() string
    SetExtraHeaders([]sippy_header.SipHeader)
    GetAcct(*sippy_time.MonoTime) (time.Duration, time.Duration, bool, bool)
    GetCLI() string
    GetCLD() string
    GetUasLossEmul() int
    UasLossEmul() int
    BeforeRequestSent(SipRequest)
    BeforeResponseSent(SipResponse)
    PrepTr(SipRequest) (ClientTransaction, error)
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
    SetOutboundProxy(*sippy_net.HostPort)
    SetAckRparams(*sippy_net.HostPort, *sippy_header.SipURL, []*sippy_header.SipRoute)
    Cancel(...sippy_header.SipHeader)
    GetACK() SipRequest
    SendACK()
    SetUAck(bool)
    BeforeRequestSent(SipRequest)
    TransmitData()
    SetOnSendComplete(func())
}

type ServerTransaction interface {
    baseTransaction
    IncomingRequest(req SipRequest, checksum string)
    TimersAreActive() bool
    SetCancelCB(func(*sippy_time.MonoTime, SipRequest))
    SetNoackCB(func(*sippy_time.MonoTime))
    SendResponse(resp SipResponse, retrans bool, ack_cb func(SipRequest))
    SendResponseWithLossEmul(resp SipResponse, retrans bool, ack_cb func(SipRequest), lossemul int)
    Cleanup()
    UpgradeToSessionLock(sync.Locker)
    SetServer(*sippy_header.SipServer)
    SetBeforeResponseSent(func(SipResponse))
    Setup100rel(SipRequest)
}

type SipTransactionManager interface {
    RegConsumer(UA, string)
    UnregConsumer(UA, string)
    BeginNewClientTransaction(SipRequest, ResponseReceiver, sync.Locker, *sippy_net.HostPort, sippy_net.Transport, func(SipRequest))
    CreateClientTransaction(SipRequest, ResponseReceiver, sync.Locker, *sippy_net.HostPort, sippy_net.Transport, func(SipRequest)) (ClientTransaction, error)
    BeginClientTransaction(SipRequest, ClientTransaction)
    SendResponse(resp SipResponse, lock bool, ack_cb func(SipRequest))
    SendResponseWithLossEmul(resp SipResponse, lock bool, ack_cb func(SipRequest), lossemul int)
    Run()
    Shutdown()
}

type UaState interface {
    RecvEvent(CCEvent) (UaState, func(), error)
    RecvResponse(SipResponse, ClientTransaction) (UaState, func())
    RecvRequest(SipRequest, ServerTransaction) (UaState, func())
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
    AppendExtraHeader(sippy_header.SipHeader)
}

type StatefulProxy interface {
    RequestReceiver
}

type OnRingingListener func(*sippy_time.MonoTime, string, int)
type OnDisconnectListener func(*sippy_time.MonoTime, string, int, SipRequest)
type OnFailureListener func(*sippy_time.MonoTime, string, int)
type OnConnectListener func(*sippy_time.MonoTime, string)
type OnDeadListener func()
type OnLocalSdpChange func(MsgBody, func(MsgBody)) error
type OnRemoteSdpChange func(MsgBody, func(MsgBody)) error

type RtpProxyClientOpts interface {
    GetNWorkers() *int
}

type RtpProxyClient interface {
    SendCommand(string, func(string))
    SBindSupported() bool
    IsLocal() bool
    TNotSupported() bool
    GetProxyAddress() string
    IsOnline() bool
    GoOnline()
    GoOffline()
    GetOpts() RtpProxyClientOpts
    Start() error
}

type RtpProxyUpdateResult interface {
    Address() string
}
