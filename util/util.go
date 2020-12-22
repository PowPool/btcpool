package util

import (
	"bytes"
	"encoding/hex"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/mutalisk999/bitcoin-lib/src/base58"
	"github.com/mutalisk999/bitcoin-lib/src/bigint"
	"github.com/mutalisk999/bitcoin-lib/src/utility"
	"math/big"
	"regexp"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/math"
)

//var Ether = math.BigPow(10, 18)
//var Shannon = math.BigPow(10, 9)

var BTC = math.BigPow(10, 8)
var Satoshi = math.BigPow(10, 0)

var pow256 = math.BigPow(2, 256)
var addressPattern = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
var zeroHash = regexp.MustCompile("^0?x?0+$")

//func IsValidHexAddress(s string) bool {
//	if IsZeroHash(s) || !addressPattern.MatchString(s) {
//		return false
//	}
//	return true
//}

func IsValidBTCAddress(address string) bool {
	addrWithCheck, err := base58.Decode(address)
	if err != nil {
		return false
	}
	if len(addrWithCheck) != 25 {
		return false
	}
	check1 := utility.Sha256(utility.Sha256(addrWithCheck[0:21]))[0:4]
	check2 := addrWithCheck[21:25]
	if bytes.Compare(check1, check2) != 0 {
		return false
	}
	return true
}

func IsZeroHash(s string) bool {
	return zeroHash.MatchString(s)
}

func MakeTimestamp() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func GetTargetHex(diff int64) string {
	difficulty := big.NewInt(diff)
	diff1 := new(big.Int).Div(pow256, difficulty)
	return string(hexutil.Encode(diff1.Bytes()))
}

func TargetHexToDiff(targetHex string) *big.Int {
	targetBytes := common.FromHex(targetHex)
	return new(big.Int).Div(pow256, new(big.Int).SetBytes(targetBytes))
}

func ToHex(n int64) string {
	return "0x0" + strconv.FormatInt(n, 16)
}

func FormatReward(reward *big.Int) string {
	return reward.String()
}

//func FormatRatReward(reward *big.Rat) string {
//	wei := new(big.Rat).SetInt(Ether)
//	reward = reward.Quo(reward, wei)
//	return reward.FloatString(8)
//}

func FormatRatReward(reward *big.Rat) string {
	satoshi := new(big.Rat).SetInt(BTC)
	reward = reward.Quo(reward, satoshi)
	return reward.FloatString(8)
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func MustParseDuration(s string) time.Duration {
	value, err := time.ParseDuration(s)
	if err != nil {
		panic("util: Can't parse duration `" + s + "`: " + err.Error())
	}
	return value
}

func String2Big(num string) *big.Int {
	n := new(big.Int)
	n.SetString(num, 0)
	return n
}

func TargetHash256StratumFormat(hexStr string) (string, error) {
	var b bigint.Uint256
	err := b.SetHex(hexStr)
	if err != nil {
		return "", err
	}
	var b2 []byte
	for i := 0; i < 8; i++ {
		for j := 0; j < 4; j++ {
			b2 = append(b2, b.GetData()[i*4+(3-j)])
		}
	}
	return hex.EncodeToString(b2), nil
}

func Hash256StratumFormat(hexStr string) (string, error) {
	var b bigint.Uint256
	err := b.SetHex(hexStr)
	if err != nil {
		return "", err
	}
	var b2 []byte
	b2 = append(b2, b.GetData()...)
	return hex.EncodeToString(b2), nil
}
