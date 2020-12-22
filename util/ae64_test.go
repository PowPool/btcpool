package util

import (
	"fmt"
	"testing"
)

func TestAe64Encode(t *testing.T) {
	src := []byte("yambfpk3cat4eA7PAXD1a8dqEZzGkiDrZ5")
	src2 := []byte("12345678")
	key := []byte("12345678")
	dst, _ := Ae64Encode(src, key)
	dst2, _ := Ae64Encode(src2, key)
	fmt.Println(dst)
	fmt.Println(dst2)
}

func TestAe64Decode(t *testing.T) {
	src := "X/anuih86ocszNESCwJw+KCDe1wce+1Dk7Q4zCBE9a/lnCwydhF7gSkmHvcdwPjk"
	src2 := "m0lxCSrfYVhmOhZcOhICrw=="
	key := []byte("12345678")
	orgi, _ := Ae64Decode(src, key)
	orgi2, _ := Ae64Decode(src2, key)
	fmt.Println(string(orgi))
	fmt.Println(string(orgi2))
}
