package proxy

import (
	"encoding/hex"
	"fmt"
	"github.com/MiningPool0826/btcpool/bitcoin"
	"github.com/mutalisk999/bitcoin-lib/src/utility"
	"regexp"
	"strconv"
	"strings"

	//"github.com/MiningPool0826/btcpool/rpc"
	. "github.com/MiningPool0826/btcpool/util"
)

var noncePattern = regexp.MustCompile("^[0-9a-f]{8}$")
var hashPattern = regexp.MustCompile("^[0-9a-f]{64}$")
var workerPattern = regexp.MustCompile("^[0-9a-zA-Z-_\x2e]{1,64}$")

// Stratum
func (s *ProxyServer) handleSubscribeRPC(cs *Session) (interface{}, *ErrorReply) {
	cs.target = s.target
	// at first time, target is the same with targetNextJob
	cs.targetNextJob = s.target
	s.registerSession(cs)
	Info.Printf("Stratum miner connected from %v", cs.ip)

	cs.sid = hex.EncodeToString(utility.Sha256(
		[]byte(strings.Join([]string{cs.ip, strconv.Itoa(int(s.config.Id)), strconv.Itoa(int(cs.tag))}, ","))))[0:32]
	cs.extraNonce1 = fmt.Sprintf("%08x", uint32(s.config.Id)<<16|uint32(cs.tag))

	setDiff := []string{"mining.set_difficulty", cs.sid}
	notify := []string{"mining.notify", cs.sid}
	l := []interface{}{setDiff, notify}
	reply := []interface{}{l, cs.extraNonce1, bitcoin.EXTRANONCE2_SIZE}

	return reply, nil
}

func (s *ProxyServer) handleAuthorizeRPC(cs *Session, params []string) (bool, *ErrorReply) {
	if len(params) == 0 {
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	l := strings.Split(strings.Trim(params[0], " \t\r\n"), ".")
	if !IsValidBTCAddress(l[0]) {
		return false, &ErrorReply{Code: -1, Message: "Invalid authorize"}
	}
	if !s.policy.ApplyLoginPolicy(l[0], cs.ip) {
		return false, &ErrorReply{Code: -1, Message: "You are blacklisted"}
	}

	id := "default"
	if len(l) > 1 && workerPattern.MatchString(l[1]) {
		id = l[1]
	}

	cs.login = l[0]
	cs.id = id
	cs.isAuth = true

	Info.Printf("Stratum miner connected %v.%v@%v", cs.login, cs.id, cs.ip)
	return true, nil
}

// Stratum
func (s *ProxyServer) handleTCPSubmitRPC(cs *Session, params []string) (bool, *ErrorReply) {
	s.sessionsMu.RLock()
	_, ok := s.sessions[cs]
	s.sessionsMu.RUnlock()

	if !ok {
		return false, &ErrorReply{Code: 25, Message: "Not subscribed"}
	}
	return s.handleSubmitRPC(cs, params)
}

func (s *ProxyServer) handleSubmitRPC(cs *Session, params []string) (bool, *ErrorReply) {
	if len(params) != 5 {
		s.policy.ApplyMalformedPolicy(cs.ip)
		Error.Printf("Malformed params from %s@%s %v", cs.login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Invalid params"}
	}

	if !noncePattern.MatchString(params[2]) || !noncePattern.MatchString(params[3]) || !noncePattern.MatchString(params[4]) {
		s.policy.ApplyMalformedPolicy(cs.ip)
		Error.Printf("Malformed PoW result from %s@%s %v", cs.login, cs.ip, params)
		return false, &ErrorReply{Code: -1, Message: "Malformed PoW result"}
	}
	t := s.currentBlockTemplate()
	exist, validShare := s.processShare(cs.login, cs.id, cs.extraNonce1, cs.ip, TargetHexToDiff(cs.target).Int64(), t, params)
	ok := s.policy.ApplySharePolicy(cs.ip, !exist && validShare)

	if exist {
		Error.Printf("Duplicate share from %s@%s %v", cs.login, cs.ip, params)
		ShareLog.Printf("Duplicate share from %s@%s %v", cs.login, cs.ip, params)
		return false, &ErrorReply{Code: 22, Message: "Duplicate share"}
	}

	if !validShare {
		Error.Printf("Invalid share from %s.%s@%s", cs.login, cs.id, cs.ip)
		ShareLog.Printf("Invalid share from %s.%s@%s", cs.login, cs.id, cs.ip)
		// Bad shares limit reached, return error and close
		if !ok {
			return false, &ErrorReply{Code: 23, Message: "Invalid share"}
		}
		return false, nil
	}
	Info.Printf("Valid share from %s.%s@%s", cs.login, cs.id, cs.ip)
	ShareLog.Printf("Valid share from %s.%s@%s", cs.login, cs.id, cs.ip)

	if !ok {
		return true, &ErrorReply{Code: -1, Message: "High rate of invalid shares"}
	}

	return true, nil
}

//func (s *ProxyServer) handleGetBlockByNumberRPC() *rpc.GetBlockReplyPart {
//	t := s.currentBlockTemplate()
//	var reply *rpc.GetBlockReplyPart
//	if t != nil {
//		reply = t.GetPendingBlockCache
//	}
//	return reply
//}

func (s *ProxyServer) handleUnknownRPC(cs *Session, m string) *ErrorReply {
	Error.Printf("Unknown request method %s from %s", m, cs.ip)
	s.policy.ApplyMalformedPolicy(cs.ip)
	return &ErrorReply{Code: -3, Message: "Method not found"}
}
