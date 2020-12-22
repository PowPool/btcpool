package payouts

import (
	"math/big"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCalculateRewards(t *testing.T) {
	blockReward, _ := new(big.Rat).SetString("5000000000000000000")
	shares := map[string]int64{"0x0": 1000000, "0x1": 20000, "0x2": 5000, "0x3": 10, "0x4": 1}
	expectedRewards := map[string]int64{"0x0": 4877996431, "0x1": 97559929, "0x2": 24389982, "0x3": 48780, "0x4": 4878}
	totalShares := int64(1025011)

	rewards := calculateRewardsForShares(shares, totalShares, blockReward)
	expectedTotalAmount := int64(5000000000)

	totalAmount := int64(0)
	for login, amount := range rewards {
		totalAmount += amount

		if expectedRewards[login] != amount {
			t.Errorf("Amount for %v must be equal to %v vs %v", login, expectedRewards[login], amount)
		}
	}
	if totalAmount != expectedTotalAmount {
		t.Errorf("Total reward must be equal to block reward in Satoshi: %v vs %v", expectedTotalAmount, totalAmount)
	}
}

func TestChargeFee(t *testing.T) {
	orig, _ := new(big.Rat).SetString("5000000000000000000")
	value, _ := new(big.Rat).SetString("5000000000000000000")
	expectedNewValue, _ := new(big.Rat).SetString("3750000000000000000")
	expectedFee, _ := new(big.Rat).SetString("1250000000000000000")
	newValue, fee := chargeFee(orig, 25.0)

	if orig.Cmp(value) != 0 {
		t.Error("Must not change original value")
	}
	if newValue.Cmp(expectedNewValue) != 0 {
		t.Error("Must charge and deduct correct fee")
	}
	if fee.Cmp(expectedFee) != 0 {
		t.Error("Must charge fee")
	}
}
