package sippy

import (
	"sync"
	"time"
	"strconv"

	"github.com/sippy/go-b2bua/sippy/headers"
	"github.com/sippy/go-b2bua/sippy/net"
	"github.com/sippy/go-b2bua/sippy/types"
	"github.com/sippy/go-b2bua/sippy/conf"
)

type SipRegistrationAgent struct {
	dead            bool
	Rmsg            sippy_types.SipRequest
	source_address  *sippy_net.HostPort
	global_config   sippy_conf.Config
	user            sippy_types.NullString
	passw           sippy_types.NullString
	rok_cb          func(time.Time, *sippy_header.SipAddress)
	Rfail_cb        func(string)
	sip_tm          sippy_types.SipTransactionManager
	Lock            sync.Mutex
	atries          int
	AuthProvider   sippy_types.AuthProvider
}


func NewSipRegistrationAgent(global_config sippy_conf.Config, sip_tm sippy_types.SipTransactionManager, aor, contact_url *sippy_header.SipURL, user, passw sippy_types.NullString, rok_cb func(time.Time, *sippy_header.SipAddress), rfail_cb func(string), target *sippy_net.HostPort, expires_param int) *SipRegistrationAgent {
	var contact *sippy_header.SipContact
	self := &SipRegistrationAgent{
			global_config   : global_config,
			user            : user,
			passw           : passw,
			rok_cb          : rok_cb,
			Rfail_cb        : rfail_cb,
			sip_tm          : sip_tm,
	}
	ruri := aor.GetCopy()
	ruri.Username = ""
	aor.Port = nil
	to_addr := sippy_header.NewSipAddress("", aor)
	from_addr := to_addr.GetCopy()
	from := sippy_header.NewSipFrom(from_addr, global_config)
	from_addr.GenTag()
	to := sippy_header.NewSipTo(to_addr, global_config)
	if contact_url == nil {
			contact = sippy_header.NewSipContact(global_config)
	} else {
			contact = sippy_header.NewSipContactFromAddress(sippy_header.NewSipAddress("", contact_url))
	}
	contact_addr, _ := contact.GetBody(global_config)
	contact_addr.SetParam("expires", strconv.Itoa(expires_param))
	self.Rmsg, _ = NewSipRequest("REGISTER", ruri,
			"", // sipver
			to,
			from,
			nil, // via
			1, // CSeq
			nil, // callid
			nil, // maxforwards
			nil, // body
			contact,
			nil, // routes
			target,
			nil, // user_agent
			nil, // expires
			global_config)
	self.AuthProvider = self
	return self
}

func (self *SipRegistrationAgent) DoRegister() {
	if self.dead {
			return
	}
	self.sip_tm.BeginNewClientTransaction(self.Rmsg, self, &self.Lock, /*laddress*/ self.source_address, nil, nil)
	via_hdr, err := self.Rmsg.GetVias()[0].GetBody()
	if err == nil {
			via_hdr.GenBranch()
	}
	cseq, err := self.Rmsg.GetCSeq().GetBody()
	if err == nil {
			cseq.CSeq++
	}
}

func (self *SipRegistrationAgent) StopRegister() {
	self.dead = true
	self.Rmsg = nil
}

func (self *SipRegistrationAgent) HandleAuth(challenges []sippy_types.Challenge) {
	auth, err := challenges[0].GenAuthHF(self.user.String, self.passw.String, "REGISTER", self.Rmsg.GetRURI().String(), "")
	if err == nil {
			self.AuthDone(auth)
	}
}


func (self *SipRegistrationAgent) AuthDone(auth sippy_header.SipHeader) {
	self.Rmsg.AppendHeader(auth)
	self.atries += 1
	self.DoRegister()
}

func (self *SipRegistrationAgent) RecvResponse(resp sippy_types.SipResponse, tr sippy_types.ClientTransaction) {
	var err error

	if self.dead {
			return
	}
	if resp.GetSCodeNum() < 200 {
			return
	}
	if resp.GetSCodeNum() >= 200 && resp.GetSCodeNum() < 300 && resp.GetSCodeReason() != "Auth Failed" {
			tout := -1
			var contact *sippy_header.SipAddress

			if len(resp.GetContacts()) > 0 {
					contact, err = resp.GetContacts()[0].GetBody(self.global_config)
					if err != nil {
							tout = -1
					} else {
							tout, err = strconv.Atoi(contact.GetParam("expires"))
							if err != nil {
									tout = -1
							}
					}
			}
			if tout == -1 {
					hf := resp.GetFirstHF("expires")
					if expires_hf, ok := hf.(*sippy_header.SipExpires); ok {
							tout = expires_hf.Number
					}
			}
			if tout == -1 {
					tout = 180
			}
			expires := time.Duration(tout) * time.Second
			StartTimeout(self.DoRegister, &self.Lock, expires, 1, self.global_config.ErrorLogger())
			if self.rok_cb != nil {
					self.rok_cb(time.Now().Add(expires), contact)
			}
			self.atries = 0
			return
	}
	if (resp.GetSCodeNum() == 401 || resp.GetSCodeNum() == 407) && self.user.Valid && self.passw.Valid && self.atries < 3 {
			challenges := resp.GetChallenges()
			if len(challenges) > 0 {
					self.AuthProvider.HandleAuth(challenges)
					return
			}
	}
	if self.Rfail_cb != nil {
			self.Rfail_cb(resp.GetSL())
	}
	StartTimeout(self.DoRegister, &self.Lock, 60 * time.Second, 1, self.global_config.ErrorLogger())
	self.atries = 0
}
