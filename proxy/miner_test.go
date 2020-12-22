package proxy

import (
	"encoding/hex"
	"fmt"
	"github.com/mutalisk999/bitcoin-lib/src/utility"
	"testing"
)

func TestDoubleSha256Hash(t *testing.T) {
	//"adf6e2e56df692822f5e064a8b6404a05d67cccd64bc90f57f65b46805e9a54b"
	b1, _ := hex.DecodeString("01000000f615f7ce3b4fc6b8f61e8f89aedb1d0852507650533a9e3b10b9bbcc30639f279fcaa86746e1ef52d3edb3c4ad8259920d509bd073605c9bf1d59983752a6b06b817bb4ea78e011d012d59d4")
	b1h := utility.Sha256(utility.Sha256(b1))
	b1r := make([]byte, len(b1h))
	for i := 0; i < len(b1h); i++ {
		b1r[i] = b1h[len(b1h)-i-1]
	}
	fmt.Println("b1r: ", hex.EncodeToString(b1r))
}
