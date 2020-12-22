package util

import (
	"fmt"
	"testing"
)

func TestTargetHash256StratumFormat(t *testing.T) {
	//d37214c56841e40e6e7b504605d83c034011f715279b3d500000001600000000
	blockHashHex := "0000000000000016279b3d504011f71505d83c036e7b50466841e40ed37214c5"
	blockHashStratumHex, _ := TargetHash256StratumFormat(blockHashHex)
	fmt.Println("blockHashStratumHex:", blockHashStratumHex)
}

func TestHash256StratumFormat(t *testing.T) {
	//533eddad5a8998679ce9d8ec8692b9a37ecff0fb0ca5f028e6ac85a0f6a38b87
	txIdHex := "878ba3f6a085ace628f0a50cfbf0cf7ea3b99286ecd8e99c6798895aaddd3e53"
	txIdStratumHex, _ := Hash256StratumFormat(txIdHex)
	fmt.Println("txIdStratumHex:", txIdStratumHex)
}
