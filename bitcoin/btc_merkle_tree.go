package bitcoin

import (
	"errors"
	"github.com/mutalisk999/bitcoin-lib/src/bigint"
	"github.com/mutalisk999/bitcoin-lib/src/utility"
)

func NilTxId() bigint.Uint256 {
	var nilTxId bigint.Uint256
	_ = nilTxId.SetHex("0000000000000000000000000000000000000000000000000000000000000000")
	return nilTxId
}

type TransactionMerkleTree struct {
	rawTxIdsWithoutCoinBase     []bigint.Uint256
	txIdCoinBase                bigint.Uint256
	merkleBranchWithoutCoinBase []bigint.Uint256
}

func (t *TransactionMerkleTree) AppendNilTxId() {
	t.rawTxIdsWithoutCoinBase = append(t.rawTxIdsWithoutCoinBase, NilTxId())
}

func (t *TransactionMerkleTree) AppendTxIdsWithoutCoinBase(txIds []bigint.Uint256) {
	t.rawTxIdsWithoutCoinBase = append(t.rawTxIdsWithoutCoinBase, txIds...)
}

func (t *TransactionMerkleTree) AppendTxIdsHexWithoutCoinBase(txIdsHex []string) error {
	for _, txIdHex := range txIdsHex {
		var txId bigint.Uint256
		err := txId.SetHex(txIdHex)
		if err != nil {
			return err
		}
		t.rawTxIdsWithoutCoinBase = append(t.rawTxIdsWithoutCoinBase, txId)
	}
	return nil
}

func (t *TransactionMerkleTree) SetTxIdCoinBase(txIdCoinBase bigint.Uint256) {
	t.txIdCoinBase = txIdCoinBase
}

func (t *TransactionMerkleTree) SetTxIdHexCoinBase(txIdCoinBaseHex string) error {
	var txId bigint.Uint256
	err := txId.SetHex(txIdCoinBaseHex)
	if err != nil {
		return err
	}
	t.SetTxIdCoinBase(txId)
	return nil
}

func (t *TransactionMerkleTree) SetMerkleBranchWithoutCoinBase(merkleBranch []bigint.Uint256) {
	t.merkleBranchWithoutCoinBase = merkleBranch
}

func (t *TransactionMerkleTree) SetMerkleBranchHexWithoutCoinBase(merkleBranchHex []string) error {
	for _, hashHex := range merkleBranchHex {
		var hash bigint.Uint256
		err := hash.SetHex(hashHex)
		if err != nil {
			return err
		}
		t.merkleBranchWithoutCoinBase = append(t.merkleBranchWithoutCoinBase, hash)
	}
	return nil
}

func (t *TransactionMerkleTree) UpdateMerkleBranch() {
	if len(t.merkleBranchWithoutCoinBase) != 0 {
		t.merkleBranchWithoutCoinBase = []bigint.Uint256{}
	}

	var preList []bigint.Uint256
	preList = append(preList, NilTxId())

	var startIndex = 2

	list := t.rawTxIdsWithoutCoinBase
	var listLength = len(list)

	if listLength > 1 {
		for {
			if listLength == 1 {
				break
			}
			t.merkleBranchWithoutCoinBase = append(t.merkleBranchWithoutCoinBase, list[1])

			if listLength%2 == 1 {
				list = append(list, list[len(list)-1])
			}

			newList := preList
			for i := startIndex; i < len(list); i = i + 2 {
				bytes1 := list[i].GetData()
				bytes2 := list[i+1].GetData()
				bytesAll := append(append([]byte{}, bytes1...), bytes2...)
				bytesDoubleSha := utility.Sha256(utility.Sha256(bytesAll))
				var doubleSha bigint.Uint256
				doubleSha.SetData(bytesDoubleSha)
				newList = append(newList, doubleSha)
			}
			list = newList
			listLength = len(list)
		}
	}
}

func (t TransactionMerkleTree) GetMerkleBranch() []bigint.Uint256 {
	return t.merkleBranchWithoutCoinBase
}

func (t TransactionMerkleTree) GetMerkleBranchHex() []string {
	var merkleBranchHex []string
	for _, h := range t.merkleBranchWithoutCoinBase {
		merkleBranchHex = append(merkleBranchHex, h.GetHex())
	}
	return merkleBranchHex
}

func (t TransactionMerkleTree) CalcMerkleRoot() (bigint.Uint256, error) {
	bytesMerkleRoot := t.txIdCoinBase.GetData()
	for _, h := range t.merkleBranchWithoutCoinBase {
		bytes2 := h.GetData()
		bytesAll := append(append([]byte{}, bytesMerkleRoot...), bytes2...)
		bytesMerkleRoot = utility.Sha256(utility.Sha256(bytesAll))
	}
	var merkleRoot bigint.Uint256
	err := merkleRoot.SetData(bytesMerkleRoot)
	if err != nil {
		return NilTxId(), err
	}
	return merkleRoot, nil
}

func (t TransactionMerkleTree) CalcMerkleRootHex() (string, error) {
	merkleRoot, err := t.CalcMerkleRoot()
	if err != nil {
		return "", err
	}
	return merkleRoot.GetHex(), nil
}

func GetMerkleBranchHexFromTxIdsWithoutCoinBase(txIdsHexWithoutCoinBase []string) ([]string, error) {
	var tree TransactionMerkleTree
	tree.AppendNilTxId()
	err := tree.AppendTxIdsHexWithoutCoinBase(txIdsHexWithoutCoinBase)
	if err != nil {
		return nil, err
	}
	tree.UpdateMerkleBranch()
	return tree.GetMerkleBranchHex(), nil
}

func GetMerkleRootHexFromTxIdsWithCoinBase(txIdsHexWithCoinBase []string) (string, error) {
	var tree TransactionMerkleTree

	if len(txIdsHexWithCoinBase) < 1 {
		return "", errors.New("txIdsHexWithCoinBase count must greater than 1")
	}
	tree.AppendNilTxId()
	err := tree.AppendTxIdsHexWithoutCoinBase(txIdsHexWithCoinBase[1:])
	if err != nil {
		return "", err
	}
	tree.UpdateMerkleBranch()

	err = tree.SetTxIdHexCoinBase(txIdsHexWithCoinBase[0])
	if err != nil {
		return "", err
	}
	merkleRootHex, err := tree.CalcMerkleRootHex()
	if err != nil {
		return "", err
	}
	return merkleRootHex, nil
}

func GetMerkleRootHexFromCoinBaseAndMerkleBranch(txIdCoinBaseHex string, merkleBranchHex []string) (string, error) {
	var tree TransactionMerkleTree

	err := tree.SetTxIdHexCoinBase(txIdCoinBaseHex)
	if err != nil {
		return "", err
	}
	err = tree.SetMerkleBranchHexWithoutCoinBase(merkleBranchHex)
	if err != nil {
		return "", err
	}
	merkleRootHex, err := tree.CalcMerkleRootHex()
	if err != nil {
		return "", err
	}
	return merkleRootHex, nil
}
