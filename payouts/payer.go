package payouts

import (
	"time"

	"github.com/MiningPool0826/btcpool/rpc"
	"github.com/MiningPool0826/btcpool/storage"
)

const txCheckInterval = 5 * time.Second

type PayoutsConfig struct {
	Enabled      bool   `json:"enabled"`
	RequirePeers int64  `json:"requirePeers"`
	Interval     string `json:"interval"`
	Daemon       string `json:"daemon"`
	Timeout      string `json:"timeout"`
	Address      string `json:"address"`
	Gas          string `json:"gas"`
	GasPrice     string `json:"gasPrice"`
	AutoGas      bool   `json:"autoGas"`
	// In Shannon
	Threshold int64 `json:"threshold"`
	BgSave    bool  `json:"bgsave"`
}

//func (c PayoutsConfig) GasHex() string {
//	x := String2Big(c.Gas)
//	return hexutil.EncodeBig(x)
//}
//
//func (c PayoutsConfig) GasPriceHex() string {
//	x := String2Big(c.GasPrice)
//	return hexutil.EncodeBig(x)
//}

type PayoutsProcessor struct {
	config   *PayoutsConfig
	backend  *storage.RedisClient
	rpc      *rpc.RPCClient
	halt     bool
	lastFail error
}

//func NewPayoutsProcessor(cfg *PayoutsConfig, backend *storage.RedisClient) *PayoutsProcessor {
//	u := &PayoutsProcessor{config: cfg, backend: backend}
//	u.rpc = rpc.NewRPCClient("PayoutsProcessor", cfg.Daemon, cfg.Timeout)
//	return u
//}

//func (p *PayoutsProcessor) Start() {
//	Info.Println("Starting payouts")
//
//	if p.mustResolvePayout() {
//		Info.Println("Running with env RESOLVE_PAYOUT=1, now trying to resolve locked payouts")
//		p.resolvePayouts()
//		Info.Println("Now you have to restart payouts module with RESOLVE_PAYOUT=0 for normal run")
//		return
//	}
//
//	intv := MustParseDuration(p.config.Interval)
//	timer := time.NewTimer(intv)
//	Info.Printf("Set payouts interval to %v", intv)
//
//	payments := p.backend.GetPendingPayments()
//	if len(payments) > 0 {
//		Error.Printf("Previous payout failed, you have to resolve it. List of failed payments:\n %v", formatPendingPayments(payments))
//		return
//	}
//
//	locked, err := p.backend.IsPayoutsLocked()
//	if err != nil {
//		Error.Println("Unable to start payouts:", err)
//		return
//	}
//	if locked {
//		Error.Println("Unable to start payouts because they are locked")
//		return
//	}
//
//	// Immediately process payouts after start
//	p.process()
//	timer.Reset(intv)
//
//	go func() {
//		for {
//			select {
//			case <-timer.C:
//				p.process()
//				timer.Reset(intv)
//			}
//		}
//	}()
//}

//func (p *PayoutsProcessor) process() {
//	if p.halt {
//		Info.Println("Payments suspended due to last critical error:", p.lastFail)
//		return
//	}
//	mustPay := 0
//	minersPaid := 0
//	totalAmount := big.NewInt(0)
//	payees, err := p.backend.GetPayees()
//	if err != nil {
//		Error.Println("Error while retrieving payees from backend:", err)
//		return
//	}
//
//	for _, login := range payees {
//		amount, _ := p.backend.GetBalance(login)
//		amountInShannon := big.NewInt(amount)
//
//		// Shannon^2 = Wei
//		amountInWei := new(big.Int).Mul(amountInShannon, Shannon)
//
//		if !p.reachedThreshold(amountInShannon) {
//			continue
//		}
//		mustPay++
//
//		// Require active peers before processing
//		if !p.checkPeers() {
//			break
//		}
//		// Require unlocked account
//		if !p.isUnlockedAccount() {
//			break
//		}
//
//		// Check if we have enough funds
//		poolBalance, err := p.rpc.GetBalance(p.config.Address)
//		if err != nil {
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//		if poolBalance.Cmp(amountInWei) < 0 {
//			err := fmt.Errorf("Not enough balance for payment, need %s Wei, pool has %s Wei",
//				amountInWei.String(), poolBalance.String())
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//
//		// Lock payments for current payout
//		err = p.backend.LockPayouts(login, amount)
//		if err != nil {
//			Error.Printf("Failed to lock payment for %s: %v", login, err)
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//		Info.Printf("Locked payment for %s, %v Shannon", login, amount)
//
//		// Debit miner's balance and update stats
//		err = p.backend.UpdateBalance(login, amount)
//		if err != nil {
//			Error.Printf("Failed to update balance for %s, %v Shannon: %v", login, amount, err)
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//
//		value := hexutil.EncodeBig(amountInWei)
//		txHash, err := p.rpc.SendTransaction(p.config.Address, login, p.config.GasHex(), p.config.GasPriceHex(), value, p.config.AutoGas)
//		if err != nil {
//			Error.Printf("Failed to send payment to %s, %v Shannon: %v. Check outgoing tx for %s in block explorer and docs/PAYOUTS.md",
//				login, amount, err, login)
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//
//		// Log transaction hash
//		err = p.backend.WritePayment(login, txHash, amount)
//		if err != nil {
//			Error.Printf("Failed to log payment data for %s, %v Shannon, tx: %s: %v", login, amount, txHash, err)
//			p.halt = true
//			p.lastFail = err
//			break
//		}
//
//		minersPaid++
//		totalAmount.Add(totalAmount, big.NewInt(amount))
//		Info.Printf("Paid %v Shannon to %v, TxHash: %v", amount, login, txHash)
//
//		// Wait for TX confirmation before further payouts
//		for {
//			Info.Printf("Waiting for tx confirmation: %v", txHash)
//			time.Sleep(txCheckInterval)
//			receipt, err := p.rpc.GetTxReceipt(txHash)
//			if err != nil {
//				Error.Printf("Failed to get tx receipt for %v: %v", txHash, err)
//				continue
//			}
//			// Tx has been mined
//			if receipt != nil && receipt.Confirmed() {
//				if receipt.Successful() {
//					Info.Printf("Payout tx successful for %s: %s", login, txHash)
//				} else {
//					Error.Printf("Payout tx failed for %s: %s. Address contract throws on incoming tx.", login, txHash)
//				}
//				break
//			}
//		}
//	}
//
//	if mustPay > 0 {
//		Info.Printf("Paid total %v Shannon to %v of %v payees", totalAmount, minersPaid, mustPay)
//	} else {
//		Error.Println("No payees that have reached payout threshold")
//	}
//
//	// Save redis state to disk
//	if minersPaid > 0 && p.config.BgSave {
//		p.bgSave()
//	}
//}

//func (p PayoutsProcessor) isUnlockedAccount() bool {
//	_, err := p.rpc.Sign(p.config.Address, "0x0")
//	if err != nil {
//		Error.Println("Unable to process payouts:", err)
//		return false
//	}
//	return true
//}

//func (p PayoutsProcessor) checkPeers() bool {
//	n, err := p.rpc.GetPeerCount()
//	if err != nil {
//		Error.Println("Unable to start payouts, failed to retrieve number of peers from node:", err)
//		return false
//	}
//	if n < p.config.RequirePeers {
//		Error.Println("Unable to start payouts, number of peers on a node is less than required", p.config.RequirePeers)
//		return false
//	}
//	return true
//}

//func (p PayoutsProcessor) reachedThreshold(amount *big.Int) bool {
//	return big.NewInt(p.config.Threshold).Cmp(amount) < 0
//}
//
//func formatPendingPayments(list []*storage.PendingPayment) string {
//	var s string
//	for _, v := range list {
//		s += fmt.Sprintf("\tAddress: %s, Amount: %v Shannon, %v\n", v.Address, v.Amount, time.Unix(v.Timestamp, 0))
//	}
//	return s
//}
//
//func (p PayoutsProcessor) bgSave() {
//	result, err := p.backend.BgSave()
//	if err != nil {
//		Error.Println("Failed to perform BGSAVE on backend:", err)
//		return
//	}
//	Info.Println("Saving backend state to disk:", result)
//}
//
//func (p PayoutsProcessor) resolvePayouts() {
//	payments := p.backend.GetPendingPayments()
//
//	if len(payments) > 0 {
//		Info.Printf("Will credit back following balances:\n%s", formatPendingPayments(payments))
//
//		for _, v := range payments {
//			err := p.backend.RollbackBalance(v.Address, v.Amount)
//			if err != nil {
//				Error.Printf("Failed to credit %v Shannon back to %s, error is: %v", v.Amount, v.Address, err)
//				return
//			}
//			Info.Printf("Credited %v Shannon back to %s", v.Amount, v.Address)
//		}
//		err := p.backend.UnlockPayouts()
//		if err != nil {
//			Error.Println("Failed to unlock payouts:", err)
//			return
//		}
//	} else {
//		Error.Println("No pending payments to resolve")
//	}
//
//	if p.config.BgSave {
//		p.bgSave()
//	}
//	Info.Println("Payouts unlocked")
//}
//
//func (p PayoutsProcessor) mustResolvePayout() bool {
//	v, _ := strconv.ParseBool(os.Getenv("RESOLVE_PAYOUT"))
//	return v
//}
