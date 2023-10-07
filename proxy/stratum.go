package proxy

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/PowPool/btcpool/bitcoin"
	"github.com/mutalisk999/bitcoin-lib/src/bigint"
	"io"
	"net"
	"time"

	. "github.com/PowPool/btcpool/util"
)

const (
	MaxReqSize = 1024
)

func (s *ProxyServer) ListenTCP() {
	timeout := MustParseDuration(s.config.Proxy.Stratum.Timeout)
	s.timeout = timeout

	addr, err := net.ResolveTCPAddr("tcp", s.config.Proxy.Stratum.Listen)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}
	server, err := net.ListenTCP("tcp", addr)
	if err != nil {
		Error.Fatalf("Error: %v", err)
	}
	defer server.Close()

	Info.Printf("Stratum listening on %s", s.config.Proxy.Stratum.Listen)
	var accept = make(chan int, s.config.Proxy.Stratum.MaxConn)

	tag := 0
	for i := 0; i < s.config.Proxy.Stratum.MaxConn; i++ {
		accept <- i
	}

	for {
		conn, err := server.AcceptTCP()
		if err != nil {
			continue
		}
		Info.Println("Accept Stratum TCP Connection from: ", conn.RemoteAddr().String())

		_ = conn.SetKeepAlive(true)

		ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

		if s.policy.IsBanned(ip) || !s.policy.ApplyLimitPolicy(ip) {
			_ = conn.Close()
			continue
		}

		tag = <-accept
		cs := &Session{conn: conn, ip: ip, shareCountInv: 0, tag: uint16(tag), isAuth: false}

		go func(cs *Session, tag int) {
			err = s.handleTCPClient(cs)
			if err != nil {
				s.removeSession(cs)
				_ = conn.Close()
			}
			accept <- tag
		}(cs, tag)
	}
}

func (s *ProxyServer) handleTCPClient(cs *Session) error {
	cs.enc = json.NewEncoder(cs.conn)
	connBuf := bufio.NewReaderSize(cs.conn, MaxReqSize)
	s.setDeadline(cs.conn)

	for {
		data, isPrefix, err := connBuf.ReadLine()
		if isPrefix {
			Error.Printf("Socket flood detected from %s", cs.ip)
			s.policy.BanClient(cs.ip)
			return err
		} else if err == io.EOF {
			Info.Printf("Client %s disconnected", cs.ip)
			s.removeSession(cs)
			_ = cs.conn.Close()
			break
		} else if err != nil {
			Error.Printf("Error reading from socket: %v", err)
			Error.Printf("Address: [%s] | Name: [%s] | IP: [%s]", cs.login, cs.id, cs.ip)
			return err
		}

		if len(data) > 1 {
			var req StratumReq
			err = json.Unmarshal(data, &req)
			if err != nil {
				s.policy.ApplyMalformedPolicy(cs.ip)
				Error.Printf("handleTCPClient: Malformed stratum request from %s: %v", cs.ip, err)
				return err
			}

			s.setDeadline(cs.conn)
			err = cs.handleTCPMessage(s, &req)
			if err != nil {
				Error.Printf("handleTCPMessage: %v", err)
				return err
			}
		}
	}
	return nil
}

func (cs *Session) handleTCPMessage(s *ProxyServer, req *StratumReq) error {
	// Handle RPC methods
	switch req.Method {

	case "mining.subscribe":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (mining.subscribe) params from", cs.ip)
			return err
		}
		if len(params) > 0 {
			Info.Println("mining.subscribe:", params[0])
		}
		reply, errReply := s.handleSubscribeRPC(cs)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, reply)

	case "mining.authorize":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (mining.authorize) params from", cs.ip)
			return err
		}
		reply, errReply := s.handleAuthorizeRPC(cs, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}

		//set difficulty
		go func(s *ProxyServer, cs *Session) {
			err := cs.setDifficulty()
			if err != nil {
				Error.Printf("set difficulty error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			}
		}(s, cs)

		return cs.sendTCPResult(req.Id, reply)

	case "mining.submit":
		var params []string
		err := json.Unmarshal(req.Params, &params)
		if err != nil {
			Error.Println("Malformed stratum request (mining.submit) params from", cs.ip)
			return err
		}

		Debug.Printf("mining.submit, Param: %v", params)

		reply, errReply := s.handleTCPSubmitRPC(cs, params)
		if errReply != nil {
			return cs.sendTCPError(req.Id, errReply)
		}
		return cs.sendTCPResult(req.Id, reply)

	case "mining.extranonce.subscribe":
		return cs.sendTCPResult(req.Id, true)

	default:
		errReply := s.handleUnknownRPC(cs, req.Method)
		return cs.sendTCPError(req.Id, errReply)
	}
}

func (cs *Session) sendTCPResult(id json.RawMessage, result interface{}) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: nil, Result: result}
	return cs.enc.Encode(&message)
}

func (cs *Session) setDifficulty() error {
	cs.Lock()
	defer cs.Unlock()
	genesisWork, err := bitcoin.GetGenesisTargetWork()
	if err != nil {
		return err
	}

	diff := TargetHexToDiff(cs.targetNextJob).Int64()
	setDiff := float64(diff) / genesisWork

	message := JSONPushMessage{Id: nil, Method: "mining.set_difficulty", Params: []interface{}{setDiff}}
	return cs.enc.Encode(&message)
}

func (cs *Session) pushNewJob(params []interface{}) error {
	cs.Lock()
	defer cs.Unlock()
	message := JSONPushMessage{Id: nil, Method: "mining.notify", Params: params}
	return cs.enc.Encode(&message)
}

func (cs *Session) sendTCPError(id json.RawMessage, reply *ErrorReply) error {
	cs.Lock()
	defer cs.Unlock()

	message := JSONRpcResp{Id: id, Version: "2.0", Error: reply}
	err := cs.enc.Encode(&message)
	if err != nil {
		return err
	}
	return errors.New(reply.Message)
}

func (s *ProxyServer) setDeadline(conn *net.TCPConn) {
	_ = conn.SetDeadline(time.Now().Add(s.timeout))
}

func (s *ProxyServer) registerSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	s.sessions[cs] = struct{}{}
}

func (s *ProxyServer) removeSession(cs *Session) {
	s.sessionsMu.Lock()
	defer s.sessionsMu.Unlock()
	delete(s.sessions, cs)
}

func (s *ProxyServer) broadcastNewJobs() {
	t := s.currentBlockTemplate()
	if t == nil || len(t.PrevHash) == 0 || s.isSick() {
		return
	}
	var params []interface{}

	// reverse prev hash in bytes
	var prevHash bigint.Uint256
	err := prevHash.SetHex(t.PrevHash)
	if err != nil {
		return
	}

	prevHashHex := prevHash.GetHex()
	prevHashHexStratum, err := TargetHash256StratumFormat(prevHashHex)
	if err != nil {
		return
	}

	tplJob, ok := t.BlockTplJobMap[t.lastBlkTplId]
	if !ok {
		return
	}

	// https://stackoverflow.com/questions/44119793/why-does-json-encoding-an-empty-array-in-code-return-null
	// var MerkleBranchStratum []string
	MerkleBranchStratum := make([]string, 0)
	for _, hashHex := range tplJob.MerkleBranch {
		hashHexStratum, err := Hash256StratumFormat(hashHex)
		if err != nil {
			return
		}
		MerkleBranchStratum = append(MerkleBranchStratum, hashHexStratum)
	}

	params = append(append(append(append(append(params, t.lastBlkTplId), prevHashHexStratum), tplJob.CoinBase1), tplJob.CoinBase2), MerkleBranchStratum)
	params = append(append(append(params, fmt.Sprintf("%08x", t.Version)),
		fmt.Sprintf("%08x", t.NBits)), fmt.Sprintf("%08x", tplJob.BlkTplJobTime))
	params = append(params, t.newBlkTpl)

	s.sessionsMu.RLock()
	defer s.sessionsMu.RUnlock()

	count := len(s.sessions)
	Info.Printf("Broadcasting new job to %v stratum miners", count)

	start := time.Now()
	bcast := make(chan int, 1024)
	n := 0

	for m := range s.sessions {
		if !m.isAuth {
			continue
		}

		n++
		bcast <- n

		go func(s *ProxyServer, cs *Session) {
			err := cs.pushNewJob(params)
			<-bcast
			if err != nil {
				Error.Printf("Job transmit error to %v@%v: %v", cs.login, cs.ip, err)
				s.removeSession(cs)
			} else {
				s.setDeadline(cs.conn)
			}
		}(s, m)
	}
	Info.Printf("Jobs broadcast finished %s", time.Since(start))
}
