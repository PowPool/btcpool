package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"sync"

	. "github.com/MiningPool0826/btcpool/util"
)

type RPCClient struct {
	sync.RWMutex
	Url         string
	Name        string
	sick        bool
	sickRate    int
	successRate int
	client      *http.Client
}

type GetBlockReply struct {
	Height       uint32  `json:"height"`
	Hash         string  `json:"hash"`
	Nonce        uint32  `json:"nonce"`
	Difficulty   float64 `json:"difficulty"`
	Transactions []Tx    `json:"tx"`
}

type CoinBaseAux struct {
	Flags string `json:"flags"`
}

type BlockTplTransaction struct {
	Data string `json:"data"`
	Hash string `json:"hash"`
	Fee  int64  `json:"fee"`
}

type MasterNode struct {
	Payee  string `json:"payee"`
	Script string `json:"script"`
	Amount int64  `json:"amount"`
}

type GetBlockTemplateReplyPart struct {
	Version           uint32                `json:"version"`
	PreviousBlockHash string                `json:"previousblockhash"`
	Transactions      []BlockTplTransaction `json:"transactions"`
	CoinBaseAux       CoinBaseAux           `json:"coinbaseaux"`
	CoinBaseValue     int64                 `json:"coinbasevalue"`
	CurTime           uint32                `json:"curtime"`
	Bits              string                `json:"bits"`
	Target            string                `json:"target"`
	Height            uint32                `json:"height"`
	CoinbasePayload   string                `json:"coinbase_payload"`
	MasterNodes       []MasterNode          `json:"masternode"`
}

const receiptStatusSuccessful = "0x1"

type TxReceipt struct {
	TxHash    string `json:"transactionHash"`
	GasUsed   string `json:"gasUsed"`
	BlockHash string `json:"blockHash"`
	Status    string `json:"status"`
}

func (r *TxReceipt) Confirmed() bool {
	return len(r.BlockHash) > 0
}

// Use with previous method
func (r *TxReceipt) Successful() bool {
	if len(r.Status) > 0 {
		return r.Status == receiptStatusSuccessful
	}
	return true
}

type Tx struct {
	TxId string `json:"txid"`
	Vin  []Vin  `json:"vin"`
	Vout []Vout `json:"vout"`
}

type Vin struct {
	PrevOutHash string `json:"txid"`
	PrevOutN    uint32 `json:"vout"`
}

type Vout struct {
	Value    float64 `json:"value"`
	ValueSat int64   `json:"valueSat"`
	N        uint32  `json:"n"`
}

type JSONRpcResp struct {
	Id     *json.RawMessage       `json:"id"`
	Result *json.RawMessage       `json:"result"`
	Error  map[string]interface{} `json:"error"`
}

func NewRPCClient(name, url, timeout string) *RPCClient {
	rpcClient := &RPCClient{Name: name, Url: url}
	timeoutIntv := MustParseDuration(timeout)
	rpcClient.client = &http.Client{
		Timeout: timeoutIntv,
	}
	return rpcClient
}

func (r *RPCClient) GetPrevBlockHash() (string, error) {
	rpcResp, err := r.doPost(r.Url, "getbestblockhash", []string{})
	if err != nil {
		return "", err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (r *RPCClient) GetPendingBlock() (*GetBlockTemplateReplyPart, error) {
	rpcResp, err := r.doPost(r.Url, "getblocktemplate", []string{})
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockTemplateReplyPart
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) GetBlockHashByHeight(height int64) (string, error) {
	rpcResp, err := r.doPost(r.Url, "getblockhash", []int64{height})
	if err != nil {
		return "", err
	}
	var reply string
	err = json.Unmarshal(*rpcResp.Result, &reply)
	return reply, err
}

func (r *RPCClient) GetBlockByHash(hash string) (*GetBlockReply, error) {
	params := []interface{}{hash, 2}
	return r.getBlockBy("getblock", params)
}

func (r *RPCClient) getBlockBy(method string, params []interface{}) (*GetBlockReply, error) {
	rpcResp, err := r.doPost(r.Url, method, params)
	if err != nil {
		return nil, err
	}
	if rpcResp.Result != nil {
		var reply *GetBlockReply
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return reply, err
	}
	return nil, nil
}

func (r *RPCClient) SubmitBlock(params []interface{}) error {
	rpcResp, err := r.doPost(r.Url, "submitblock", params)
	if err != nil {
		return err
	}
	if rpcResp.Result != nil {
		var reply string
		err = json.Unmarshal(*rpcResp.Result, &reply)
		return errors.New(reply)
	}
	return nil
}

func (r *RPCClient) doPost(url string, method string, params interface{}) (*JSONRpcResp, error) {
	jsonReq := map[string]interface{}{"jsonrpc": "2.0", "method": method, "params": params, "id": 0}
	data, err := json.Marshal(jsonReq)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Length", (string)(len(data)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		r.markSick()
		return nil, err
	}
	defer resp.Body.Close()

	var rpcResp *JSONRpcResp
	err = json.NewDecoder(resp.Body).Decode(&rpcResp)
	if err != nil {
		r.markSick()
		return nil, err
	}
	if rpcResp.Error != nil {
		r.markSick()
		return nil, errors.New(rpcResp.Error["message"].(string))
	}
	return rpcResp, err
}

func (r *RPCClient) Check() bool {
	_, err := r.GetPrevBlockHash()
	if err != nil {
		return false
	}
	r.markAlive()
	return !r.Sick()
}

func (r *RPCClient) Sick() bool {
	r.RLock()
	defer r.RUnlock()
	return r.sick
}

func (r *RPCClient) markSick() {
	r.Lock()
	r.sickRate++
	r.successRate = 0
	if r.sickRate >= 5 {
		r.sick = true
	}
	r.Unlock()
}

func (r *RPCClient) markAlive() {
	r.Lock()
	r.successRate++
	if r.successRate >= 5 {
		r.sick = false
		r.sickRate = 0
		r.successRate = 0
	}
	r.Unlock()
}
