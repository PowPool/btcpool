package bitcoin

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"github.com/mutalisk999/bitcoin-lib/src/transaction"
	"io"
	"testing"
)

func TestGetCoinBaseScriptHex1(t *testing.T) {
	scriptHex, _ := GetCoinBaseScriptHex("XiB2rj7PdESyaxJVsnmjhXf9D9bYJjX7ob")
	fmt.Println("coinbaser script:", scriptHex)
}

func TestGetCoinBaseScriptHex2(t *testing.T) {
	scriptHex, _ := GetCoinBaseScriptHex("034a452d21d26c60076a30bf6701666b30d57ac09c2ff07f34e52cdba13796645d")
	fmt.Println("coinbaser script:", scriptHex)
}

func TestGetCoinBaseScriptHex3(t *testing.T) {
	scriptHex, _ := GetCoinBaseScriptHex("7mFVKKgyfRh6WokCP1UNvBEL2gCygwnACP")
	fmt.Println("coinbaser script:", scriptHex)
}

func TestPackNumber(t *testing.T) {
	s := PackNumber(0x01020304)
	fmt.Println("packed mumber:", s)
}

func TestPackString(t *testing.T) {
	s, _ := PackString("12345678")
	fmt.Println("packed string:", s)
}

func TestInitialize(t *testing.T) {
	var cbtx CoinBaseTransaction
	_ = cbtx.Initialize("XiB2rj7PdESyaxJVsnmjhXf9D9bYJjX7ob", 1607055201, 1827, 18492529212, "",
		"btcpool")

	extraNonce1 := []byte{0x0, 0x0, 0x0, 0x0}
	extraNonce2 := []byte{0x0, 0x0, 0x0, 0x0}
	bytesCoinBaseTx := append(append(append(append([]byte{}, cbtx.CoinBaseTx1...), extraNonce1...), extraNonce2...), cbtx.CoinBaseTx2...)
	fmt.Println("coinbase tx:", hex.EncodeToString(bytesCoinBaseTx))

	bytesBuf := bytes.NewBuffer(bytesCoinBaseTx)
	bufReader := io.Reader(bytesBuf)
	var trx transaction.Transaction
	_ = trx.UnPack(bufReader)
	fmt.Println("trx version:", trx.Version)
	fmt.Println("trx locktime", trx.LockTime)
	fmt.Println("trx vin size:", len(trx.Vin))
	for i := 0; i < len(trx.Vin); i++ {
		fmt.Println("vin prevout:", trx.Vin[i].PrevOut)
		fmt.Println("vin scriptsig:", trx.Vin[i].ScriptSig)
		fmt.Println("vin sequence:", trx.Vin[i].Sequence)
		fmt.Println("vin scriptwitness", trx.Vin[i].ScriptWitness)
	}
	fmt.Println("trx vout size:", len(trx.Vout))
	for i := 0; i < len(trx.Vout); i++ {
		fmt.Println("vout value", trx.Vout[i].Value)
		fmt.Println("vout scriptpubkey:", trx.Vout[i].ScriptPubKey)
	}
}

func TestRecoverToRawTransaction(t *testing.T) {
	var cbtx CoinBaseTransaction
	_ = cbtx.Initialize("XiB2rj7PdESyaxJVsnmjhXf9D9bYJjX7ob", 1607055201, 1827, 18492529212, "",
		"btcpool")

	extraNonce1Hex := "00000000"
	extraNonce2Hex := "00000000"
	trx, _ := cbtx.RecoverToRawTransaction(extraNonce1Hex, extraNonce2Hex)
	fmt.Println("trx version:", trx.Version)
	fmt.Println("trx locktime", trx.LockTime)
	fmt.Println("trx vin size:", len(trx.Vin))
	for i := 0; i < len(trx.Vin); i++ {
		fmt.Println("vin prevout:", trx.Vin[i].PrevOut)
		fmt.Println("vin scriptsig:", trx.Vin[i].ScriptSig)
		fmt.Println("vin sequence:", trx.Vin[i].Sequence)
		fmt.Println("vin scriptwitness", trx.Vin[i].ScriptWitness)
	}
	fmt.Println("trx vout size:", len(trx.Vout))
	for i := 0; i < len(trx.Vout); i++ {
		fmt.Println("vout value", trx.Vout[i].Value)
		fmt.Println("vout scriptpubkey:", trx.Vout[i].ScriptPubKey)
	}
}
