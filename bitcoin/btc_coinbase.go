package bitcoin

import (
	"bytes"
	"encoding/hex"
	"errors"
	"github.com/MiningPool0826/btcpool/rpc"
	"github.com/mutalisk999/bitcoin-lib/src/base58"
	"github.com/mutalisk999/bitcoin-lib/src/keyid"
	"github.com/mutalisk999/bitcoin-lib/src/pubkey"
	"github.com/mutalisk999/bitcoin-lib/src/script"
	"github.com/mutalisk999/bitcoin-lib/src/serialize"
	"github.com/mutalisk999/bitcoin-lib/src/transaction"
	"github.com/mutalisk999/bitcoin-lib/src/utility"
	"io"
	"time"
)

func GetCoinBaseScriptByPubKey(pubKeyHex string) ([]byte, error) {
	pubKey, err := hex.DecodeString(pubKeyHex)
	if err != nil {
		return nil, errors.New("invalid pubKeyHex")
	}
	if pubKey[0] != '\x02' && pubKey[0] != '\x03' {
		return nil, errors.New("invalid pubKeyHex")
	}

	bytesBuf := bytes.NewBuffer([]byte{})
	bufWriter := io.Writer(bytesBuf)

	var pubkey pubkey.PubKey
	err = pubkey.SetPubKeyData(pubKey)
	if err != nil {
		return nil, err
	}
	err = pubkey.Pack(bufWriter)
	if err != nil {
		return nil, errors.New("pack pubKeyHex err")
	}
	err = serialize.PackByte(bufWriter, script.OP_CHECKSIG)
	if err != nil {
		return nil, errors.New("pack byte err")
	}
	return bytesBuf.Bytes(), nil
}

func GetCoinBaseScriptByAddress(address string) ([]byte, error) {
	addrWithCheck, err := base58.Decode(address)
	if err != nil {
		return nil, errors.New("invalid address")
	}
	if len(addrWithCheck) != 25 {
		return nil, errors.New("invalid address")
	}
	check1 := utility.Sha256(utility.Sha256(addrWithCheck[0:21]))[0:4]
	check2 := addrWithCheck[21:25]
	if bytes.Compare(check1, check2) != 0 {
		return nil, errors.New("invalid address")
	}

	bytesBuf := bytes.NewBuffer([]byte{})
	bufWriter := io.Writer(bytesBuf)

	if addrWithCheck[0] == byte(0) || addrWithCheck[0] == byte(111) {
		// p2pkh
		// mainnet: 0
		// testnet: 111
		err = serialize.PackByte(bufWriter, script.OP_DUP)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
		err = serialize.PackByte(bufWriter, script.OP_HASH160)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
		var addr keyid.KeyID
		err = addr.SetKeyIDData(addrWithCheck[1:21])
		if err != nil {
			return nil, err
		}
		err = addr.Pack(bufWriter)
		if err != nil {
			return nil, errors.New("pack address err")
		}
		err = serialize.PackByte(bufWriter, script.OP_EQUALVERIFY)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
		err = serialize.PackByte(bufWriter, script.OP_CHECKSIG)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
	} else if addrWithCheck[0] == byte(5) || addrWithCheck[0] == byte(196) {
		// p2sh
		// mainnet: 5
		// testnet: 196
		err = serialize.PackByte(bufWriter, script.OP_HASH160)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
		var addr keyid.KeyID
		err = addr.SetKeyIDData(addrWithCheck[1:21])
		if err != nil {
			return nil, err
		}
		err = addr.Pack(bufWriter)
		if err != nil {
			return nil, errors.New("pack address err")
		}
		err = serialize.PackByte(bufWriter, script.OP_EQUAL)
		if err != nil {
			return nil, errors.New("pack byte err")
		}
	} else {
		return nil, errors.New("GetCoinBaseScriptByAddress: Invalid address")
	}

	return bytesBuf.Bytes(), nil
}

func GetCoinBaseScript(wallet string) ([]byte, error) {
	if len(wallet) == 66 {
		return GetCoinBaseScriptByPubKey(wallet)
	} else {
		return GetCoinBaseScriptByAddress(wallet)
	}
}

func GetCoinBaseScriptHex(wallet string) (string, error) {
	scriptHex, err := GetCoinBaseScript(wallet)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(scriptHex), nil
}

func PackNumber(num int64) []byte {
	s := []byte{0x1}
	for {
		if num <= 127 {
			break
		}
		s[0] += 1
		s = append(s, byte(num%256))
		num = num / 256
	}
	s = append(s, byte(num))
	return s
}

func PackString(str string) ([]byte, error) {
	bytesBuf := bytes.NewBuffer([]byte{})
	bufWriter := io.Writer(bytesBuf)
	err := serialize.PackCompactSize(bufWriter, uint64(len(str)))
	if err != nil {
		return nil, err
	}
	_, err = bufWriter.Write([]byte(str))
	if err != nil {
		return nil, err
	}
	return bytesBuf.Bytes(), nil
}

const (
	EXTRANONCE1_SIZE    = 4
	EXTRANONCE2_SIZE    = 4
	COINBASE_TX_VERSION = 2
)

type MasterNodeVout struct {
	Amount     int64
	VoutScript []byte
}

type CoinBaseTransaction struct {
	BlockTime       uint32
	BlockHeight     uint32
	RewardValue     int64
	MasterNodeVouts []MasterNodeVout
	CBExtras        string
	CBAuxFlag       []byte
	ExtraPayload    []byte
	VinScript1      []byte
	VinScript2      []byte
	VoutScript      []byte
	CoinBaseTx1     []byte
	CoinBaseTx2     []byte
}

func (t *CoinBaseTransaction) _generateCoinB() error {
	// pack coinb1
	bytesBuf := bytes.NewBuffer([]byte{})
	writer := io.Writer(bytesBuf)

	nVersion := int32(COINBASE_TX_VERSION)
	err := serialize.PackInt32(writer, nVersion)
	if err != nil {
		return err
	}

	// only 1 vin
	err = serialize.PackCompactSize(writer, uint64(1))
	if err != nil {
		return err
	}

	var prevOut transaction.OutPoint
	err = prevOut.Hash.SetHex("0000000000000000000000000000000000000000000000000000000000000000")
	if err != nil {
		return err
	}
	prevOut.N = 0xFFFFFFFF

	err = prevOut.Pack(writer)
	if err != nil {
		return err
	}

	vinScriptLen := len(t.VinScript1) + EXTRANONCE1_SIZE + EXTRANONCE2_SIZE + len(t.VinScript2)
	err = serialize.PackCompactSize(writer, uint64(vinScriptLen))
	if err != nil {
		return err
	}

	_, err = writer.Write(t.VinScript1)
	if err != nil {
		return err
	}

	t.CoinBaseTx1 = bytesBuf.Bytes()

	// pack coinb2
	bytesBuf = bytes.NewBuffer([]byte{})
	writer = io.Writer(bytesBuf)

	_, err = writer.Write(t.VinScript2)
	if err != nil {
		return err
	}

	// sequence
	err = serialize.PackUint32(writer, 0)
	if err != nil {
		return err
	}

	// vout count: 1 + len(t.MasterNodeVouts)
	err = serialize.PackCompactSize(writer, uint64(1+len(t.MasterNodeVouts)))
	if err != nil {
		return err
	}

	// pack master node vout
	for _, MasterNodeVout := range t.MasterNodeVouts {
		err = serialize.PackInt64(writer, MasterNodeVout.Amount)
		if err != nil {
			return err
		}

		var scriptPubKey script.Script
		scriptPubKey.SetScriptBytes(MasterNodeVout.VoutScript)
		err = scriptPubKey.Pack(writer)
		if err != nil {
			return err
		}
	}

	// pack coin base reward vout
	err = serialize.PackInt64(writer, t.RewardValue)
	if err != nil {
		return err
	}

	var scriptPubKey script.Script
	scriptPubKey.SetScriptBytes(t.VoutScript)
	err = scriptPubKey.Pack(writer)
	if err != nil {
		return err
	}

	// locktime
	err = serialize.PackUint32(writer, 0)
	if err != nil {
		return err
	}

	var scriptExtra script.Script
	scriptExtra.SetScriptBytes(t.ExtraPayload)
	err = scriptExtra.Pack(writer)
	if err != nil {
		return err
	}

	t.CoinBaseTx2 = bytesBuf.Bytes()

	return nil
}

func (t *CoinBaseTransaction) Initialize(cbWallet string, bTime uint32, height uint32, value int64, flags string,
	cbPayload string, cbExtras string, masterNodes []rpc.MasterNode) error {
	t.BlockTime = bTime
	t.BlockHeight = height
	t.RewardValue = value
	t.CBExtras = cbExtras
	cbFlag, err := hex.DecodeString(flags)
	if err != nil {
		return errors.New("hex decode CBAuxFlag error")
	}
	t.CBAuxFlag = cbFlag
	payload, err := hex.DecodeString(cbPayload)
	if err != nil {
		return errors.New("hex decode ExtraPayload error")
	}
	t.ExtraPayload = payload

	bytes1 := PackNumber(int64(t.BlockHeight))
	bytes2 := t.CBAuxFlag
	bytes3 := PackNumber(time.Now().Unix())
	bytes4 := []byte{EXTRANONCE1_SIZE + EXTRANONCE2_SIZE}
	t.VinScript1 = append(append(append(append([]byte{}, bytes1...), bytes2...), bytes3...), bytes4...)

	script2, err := PackString(t.CBExtras)
	if err != nil {
		return errors.New("pack string CBExtras error")
	}
	t.VinScript2 = script2

	t.VoutScript, err = GetCoinBaseScript(cbWallet)
	if err != nil {
		return errors.New("GetCoinBaseScript cbWallet error")
	}

	for _, masterNode := range masterNodes {
		var masterNodeVout MasterNodeVout
		masterNodeVout.Amount = masterNode.Amount
		masterNodeVout.VoutScript, err = GetCoinBaseScript(masterNode.Payee)
		if err != nil {
			return errors.New("GetCoinBaseScript masterNode.Payee error")
		}
		t.MasterNodeVouts = append(t.MasterNodeVouts, masterNodeVout)
	}

	err = t._generateCoinB()
	if err != nil {
		return errors.New("_generateCoinB error")
	}

	return nil
}

func (t *CoinBaseTransaction) RecoverToRawTransaction(extraNonce1Hex string, extraNonce2Hex string) (transaction.Transaction, error) {
	extraNonce1, err := hex.DecodeString(extraNonce1Hex)
	if err != nil {
		return transaction.Transaction{}, errors.New("decode hex extraNonce1Hex error")
	}

	extraNonce2, err := hex.DecodeString(extraNonce2Hex)
	if err != nil {
		return transaction.Transaction{}, errors.New("decode hex extraNonce2Hex error")
	}

	if len(extraNonce1) != EXTRANONCE1_SIZE {
		return transaction.Transaction{}, errors.New("invalid extraNonce1 length")
	}

	if len(extraNonce2) != EXTRANONCE2_SIZE {
		return transaction.Transaction{}, errors.New("invalid extraNonce2 length")
	}

	bytesCoinBaseTx := append(append(append(append([]byte{}, t.CoinBaseTx1...), extraNonce1...), extraNonce2...), t.CoinBaseTx2...)

	bytesBuf := bytes.NewBuffer(bytesCoinBaseTx)
	bufReader := io.Reader(bytesBuf)
	var btcTx transaction.Transaction
	err = btcTx.UnPack(bufReader)
	if err != nil {
		return transaction.Transaction{}, errors.New("RecoverToRawTransaction error")
	}

	return btcTx, nil
}
