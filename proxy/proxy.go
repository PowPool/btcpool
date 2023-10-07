package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"

	"github.com/PowPool/btcpool/policy"
	"github.com/PowPool/btcpool/rpc"
	"github.com/PowPool/btcpool/storage"
	. "github.com/PowPool/btcpool/util"
)

type ProxyServer struct {
	config             *Config
	blockTemplate      atomic.Value
	upstream           int32
	upstreams          []*rpc.RPCClient
	backend            *storage.RedisClient
	target             string
	policy             *policy.PolicyServer
	hashrateExpiration time.Duration
	failsCount         int64

	// Stratum
	sessionsMu sync.RWMutex
	sessions   map[*Session]struct{}
	timeout    time.Duration
}

type Session struct {
	ip  string
	enc *json.Encoder

	// Stratum
	sync.Mutex
	conn  *net.TCPConn
	login string
	id    string

	//lastShareTime int64
	target string

	targetNextJob string
	shareCountInv int64

	// Session tag
	tag uint16
	// Session id
	sid string
	// Session extra nonce1
	extraNonce1 string
	// authorized
	isAuth bool
}

func NewProxy(cfg *Config, backend *storage.RedisClient) *ProxyServer {
	if len(cfg.Name) == 0 {
		Error.Fatal("You must set instance name")
	}
	policyServer := policy.Start(&cfg.Proxy.Policy, backend)

	proxy := &ProxyServer{config: cfg, backend: backend, policy: policyServer}
	proxy.target = GetTargetHex(cfg.Proxy.Difficulty)

	proxy.upstreams = make([]*rpc.RPCClient, len(cfg.Upstream))
	for i, v := range cfg.Upstream {
		proxy.upstreams[i] = rpc.NewRPCClient(v.Name, v.Url, v.Timeout)
		Info.Printf("Upstream: %s => %s", v.Name, v.Url)
	}
	Info.Printf("Default upstream: %s => %s", proxy.rpc().Name, proxy.rpc().Url)

	if cfg.Proxy.Stratum.Enabled {
		proxy.sessions = make(map[*Session]struct{})
		go proxy.ListenTCP()
	}

	proxy.fetchBlockTemplate()

	proxy.hashrateExpiration = MustParseDuration(cfg.Proxy.HashrateExpiration)

	refreshIntv := MustParseDuration(cfg.Proxy.BlockRefreshInterval)
	refreshTimer := time.NewTimer(refreshIntv)
	Info.Printf("Set block refresh every %v", refreshIntv)

	checkIntv := MustParseDuration(cfg.UpstreamCheckInterval)
	checkTimer := time.NewTimer(checkIntv)

	stateUpdateIntv := MustParseDuration(cfg.Proxy.StateUpdateInterval)
	stateUpdateTimer := time.NewTimer(stateUpdateIntv)

	go func() {
		for {
			select {
			case <-refreshTimer.C:
				proxy.fetchBlockTemplate()
				refreshTimer.Reset(refreshIntv)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-checkTimer.C:
				proxy.checkUpstreams(cfg.UpstreamCoinBase)
				checkTimer.Reset(checkIntv)
			}
		}
	}()

	go func() {
		for {
			select {
			case <-stateUpdateTimer.C:
				t := proxy.currentBlockTemplate()
				if t != nil {
					err := backend.WriteNodeState(cfg.Name, t.Height, t.Difficulty)
					if err != nil {
						Info.Printf("Failed to write node state to backend: %v", err)
						proxy.markSick()
					} else {
						proxy.markOk()
					}
				}
				stateUpdateTimer.Reset(stateUpdateIntv)
			}
		}
	}()

	if cfg.Proxy.DiffAdjust.Enabled {
		diffAdjustIntv := MustParseDuration(cfg.Proxy.DiffAdjust.AdjustInv)
		diffAdjustTimer := time.NewTimer(diffAdjustIntv)
		Info.Printf("Difficulty adjust every %v", diffAdjustIntv)

		go func() {
			for {
				select {
				case <-diffAdjustTimer.C:
					proxy.UpdateAllSessionDiff()
					diffAdjustTimer.Reset(diffAdjustIntv)
				}
			}
		}()
	}

	return proxy
}

func (s *ProxyServer) Start() {
	Info.Printf("Starting proxy on %v", s.config.Proxy.Listen)
	r := mux.NewRouter()
	r.Handle("/{login:0x[0-9a-fA-F]{40}}/{id:[0-9a-zA-Z-_]{1,64}}", s)
	r.Handle("/{login:0x[0-9a-fA-F]{40}}", s)
	srv := &http.Server{
		Addr:           s.config.Proxy.Listen,
		Handler:        r,
		MaxHeaderBytes: s.config.Proxy.LimitHeadersSize,
	}
	err := srv.ListenAndServe()
	if err != nil {
		Error.Fatalf("Failed to start proxy: %v", err)
	}
}

func (s *ProxyServer) rpc() *rpc.RPCClient {
	i := atomic.LoadInt32(&s.upstream)
	return s.upstreams[i]
}

func (s *ProxyServer) checkUpstreams(coinBase string) {
	candidate := int32(0)
	backup := false

	for i, v := range s.upstreams {
		if v.Check() && !backup {
			candidate = int32(i)
			backup = true
		}
	}

	if s.upstream != candidate {
		Info.Printf("Switching to %v upstream", s.upstreams[candidate].Name)
		atomic.StoreInt32(&s.upstream, candidate)
	}
}

func (s *ProxyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		s.writeError(w, 405, "rpc: POST method required, received "+r.Method)
		return
	}
	ip := s.remoteAddr(r)
	if !s.policy.IsBanned(ip) {
		s.handleClient(w, r, ip)
	}
}

func (s *ProxyServer) DumpAllSessionNames() map[string]struct{} {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	l := make(map[string]struct{})
	for k, _ := range s.sessions {
		l[fmt.Sprintf("%s.%s", k.login, k.id)] = struct{}{}
	}
	return l
}

func (s *ProxyServer) UpdateAllSessionDiff() {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()

	for k, _ := range s.sessions {
		if k.shareCountInv > s.config.Proxy.DiffAdjust.ExpectShareCount*2 {
			// difficulty up
			diff := TargetHexToDiff(k.target).Int64()
			diff = int64(float64(diff) * 1.2)
			k.targetNextJob = GetTargetHex(diff)
			Info.Printf("Address: [%s], Name: : [%s], Target From [%s] Up to [%s]", k.login, k.id, k.target, k.targetNextJob)
		} else if k.shareCountInv < s.config.Proxy.DiffAdjust.ExpectShareCount/2 {
			// difficulty down
			diff := TargetHexToDiff(k.target).Int64()
			diff = int64(float64(diff) * 0.8)
			k.target = GetTargetHex(diff)
			Info.Printf("Address: [%s], Name: : [%s], Target From [%s] Down to [%s]", k.login, k.id, k.target, k.targetNextJob)
		}
	}
}

func (s *ProxyServer) remoteAddr(r *http.Request) string {
	if s.config.Proxy.BehindReverseProxy {
		ip := r.Header.Get("X-Forwarded-For")
		if len(ip) > 0 && net.ParseIP(ip) != nil {
			return ip
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}

func (s *ProxyServer) handleClient(w http.ResponseWriter, r *http.Request, ip string) {
	if r.ContentLength > s.config.Proxy.LimitBodySize {
		Error.Printf("Socket flood from %s", ip)
		s.policy.ApplyMalformedPolicy(ip)
		http.Error(w, "Request too large", http.StatusExpectationFailed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, s.config.Proxy.LimitBodySize)
	defer r.Body.Close()

	cs := &Session{ip: ip, enc: json.NewEncoder(w), shareCountInv: 0}
	dec := json.NewDecoder(r.Body)
	for {
		var req JSONRpcReq
		if err := dec.Decode(&req); err == io.EOF {
			break
		} else if err != nil {
			Error.Printf("Malformed request from %v: %v", ip, err)
			s.policy.ApplyMalformedPolicy(ip)
			return
		}
		cs.handleMessage(s, r, &req)
	}
}

func (cs *Session) handleMessage(s *ProxyServer, r *http.Request, req *JSONRpcReq) {
	if req.Id == nil {
		Error.Printf("Missing RPC id from %s", cs.ip)
		s.policy.ApplyMalformedPolicy(cs.ip)
		return
	}

	vars := mux.Vars(r)
	login := strings.ToLower(vars["login"])

	if !IsValidBTCAddress(login) {
		errReply := &ErrorReply{Code: -1, Message: "Invalid login"}
		_ = cs.sendError(req.Id, errReply)
		return
	}
	if !s.policy.ApplyLoginPolicy(login, cs.ip) {
		errReply := &ErrorReply{Code: -1, Message: "You are blacklisted"}
		_ = cs.sendError(req.Id, errReply)
		return
	}

	// Handle RPC methods
	switch req.Method {
	case "eth_submitWork":
		if req.Params != nil {
			var params []string
			err := json.Unmarshal(req.Params, &params)
			if err != nil {
				Error.Printf("Unable to parse params from %v", cs.ip)
				s.policy.ApplyMalformedPolicy(cs.ip)
				break
			}
			reply, errReply := s.handleSubmitRPC(cs, params)
			if errReply != nil {
				_ = cs.sendError(req.Id, errReply)
				break
			}
			_ = cs.sendResult(req.Id, &reply)
		} else {
			s.policy.ApplyMalformedPolicy(cs.ip)
			errReply := &ErrorReply{Code: -1, Message: "Malformed request"}
			_ = cs.sendError(req.Id, errReply)
		}
	case "eth_submitHashrate":
		_ = cs.sendResult(req.Id, true)
	default:
		errReply := s.handleUnknownRPC(cs, req.Method)
		_ = cs.sendError(req.Id, errReply)
	}
}

func (cs *Session) sendResult(id json.RawMessage, result interface{}) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendError(id json.RawMessage, reply *ErrorReply) error {
	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	return cs.enc.Encode(&message)
}

func (s *ProxyServer) writeError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
}

func (s *ProxyServer) currentBlockTemplate() *BlockTemplate {
	t := s.blockTemplate.Load()
	if t != nil {
		return t.(*BlockTemplate)
	} else {
		return nil
	}
}

func (s *ProxyServer) markSick() {
	atomic.AddInt64(&s.failsCount, 1)
}

func (s *ProxyServer) isSick() bool {
	x := atomic.LoadInt64(&s.failsCount)
	if s.config.Proxy.HealthCheck && x >= s.config.Proxy.MaxFails {
		return true
	}
	return false
}

func (s *ProxyServer) markOk() {
	atomic.StoreInt64(&s.failsCount, 0)
}
