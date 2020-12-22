package bitcoin

import (
	"fmt"
	"math"
	"math/big"
	"testing"
)

func TestNBits2Target(t *testing.T) {
	targetGenesis := NBits2Target(GENESISNBITS)
	fmt.Println("base 10, targetGenesis:", targetGenesis.Text(10))
	fmt.Printf("base 16, targetGenesis: %064s", targetGenesis.Text(16))
}

func TestGetTargetWork(t *testing.T) {
	targetGenesis, _ := new(big.Int).SetString("00000000ffff0000000000000000000000000000000000000000000000000000", 16)
	work, _ := GetTargetWork(targetGenesis)
	fmt.Println("Genesis work:", work)
}

func TestGetGenesisTargetWork(t *testing.T) {
	work, _ := GetGenesisTargetWork()
	fmt.Println("Genesis work:", work)
}

func TestGetNBitsDiff(t *testing.T) {
	fmt.Println("Genesis diff:", GetNBitsDiff(GENESISNBITS))
}

func TestGetTargetDiff(t *testing.T) {
	targetGenesis, _ := new(big.Int).SetString("00000000ffff0000000000000000000000000000000000000000000000000000", 16)
	diff, _ := GetTargetDiff(targetGenesis)
	fmt.Println("Genesis diff:", diff)
}

func TestGetDiffWork(t *testing.T) {
	work, _ := GetDiffWork(1.0)
	fmt.Println("Genesis work:", work)
}

func TestGetHashRateByWork(t *testing.T) {
	fmt.Printf("hashrate: %f MHash/s", GetHashRateByWork(math.Pow(2.0, 32.0), 600, "m"))
}

func TestGetHashRateByDiff(t *testing.T) {
	hashRate, _ := GetHashRateByDiff(19.16*math.Pow(10.0, 12), 600, "e")
	fmt.Printf("hashrate: %f EHash/s", hashRate)
}

func TestGetHashRateByNBits(t *testing.T) {
	hashRate, _ := GetHashRateByNBits(0x170eb156, 600, "e")
	fmt.Printf("hashrate: %f EHash/s", hashRate)
}
