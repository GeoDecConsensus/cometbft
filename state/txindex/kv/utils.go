package kv

import (
	"fmt"
	"math/big"

	idxutil "github.com/cometbft/cometbft/internal/indexer"
	cmtsyntax "github.com/cometbft/cometbft/libs/pubsub/query/syntax"
	"github.com/cometbft/cometbft/state/indexer"
	"github.com/cometbft/cometbft/types"
	"github.com/google/orderedcode"
)

type HeightInfo struct {
	heightRange     indexer.QueryRange
	height          int64
	heightEqIdx     int
	onlyHeightRange bool
	onlyHeightEq    bool
}

// IntInSlice returns true if a is found in the list.
func intInSlice(a int, list []int) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func ParseEventSeqFromEventKey(key []byte) (int64, error) {
	var (
		compositeKey, typ, eventValue string
		height                        int64
		eventSeq                      int64
	)

	remaining, err := orderedcode.Parse(string(key), &compositeKey, &eventValue, &height, &typ, &eventSeq)
	if err != nil {
		return 0, fmt.Errorf("failed to parse event key: %w", err)
	}

	if len(remaining) != 0 {
		return 0, fmt.Errorf("unexpected remainder in key: %s", remaining)
	}

	return eventSeq, nil
}

func dedupHeight(conditions []cmtsyntax.Condition) (dedupConditions []cmtsyntax.Condition, heightInfo HeightInfo) {
	heightInfo.heightEqIdx = -1
	heightRangeExists := false
	found := false
	var heightCondition []cmtsyntax.Condition
	heightInfo.onlyHeightEq = true
	heightInfo.onlyHeightRange = true
	for _, c := range conditions {
		if c.Tag == types.TxHeightKey {
			if c.Op == cmtsyntax.TEq {
				if heightRangeExists || found {
					continue
				}
				hFloat := c.Arg.Number()
				if hFloat != nil {
					h, _ := hFloat.Int64()
					heightInfo.height = h
					found = true
					heightCondition = append(heightCondition, c)
				}
			} else {
				heightInfo.onlyHeightEq = false
				heightRangeExists = true
				dedupConditions = append(dedupConditions, c)
			}
		} else {
			heightInfo.onlyHeightRange = false
			heightInfo.onlyHeightEq = false
			dedupConditions = append(dedupConditions, c)
		}
	}
	if !heightRangeExists && len(heightCondition) != 0 {
		heightInfo.heightEqIdx = len(dedupConditions)
		heightInfo.onlyHeightRange = false
		dedupConditions = append(dedupConditions, heightCondition...)
	} else {
		// If we found a range make sure we set the height idx to -1 as the height equality
		// will be removed
		heightInfo.heightEqIdx = -1
		heightInfo.height = 0
		heightInfo.onlyHeightEq = false
	}
	return dedupConditions, heightInfo
}

func checkHeightConditions(heightInfo HeightInfo, keyHeight int64) (bool, error) {
	if heightInfo.heightRange.Key != "" {
		withinBounds, err := idxutil.CheckBounds(heightInfo.heightRange, big.NewInt(keyHeight))
		if err != nil || !withinBounds {
			return false, err
		}
	} else {
		if heightInfo.height != 0 && keyHeight != heightInfo.height {
			return false, nil
		}
	}
	return true, nil
}

func getKeys(indexer *TxIndex) [][]byte {
	var keys [][]byte
	itr, err := indexer.store.Iterator(nil, nil)
	if err != nil {
		panic(err)
	}
	for ; itr.Valid(); itr.Next() {
		keys = append(keys, itr.Key())
	}
	return keys
}

func equal(x []byte, y []byte) bool {
	if len(x) != len(y) {
		return false
	}
	for i, elem := range x {
		if elem != y[i] {
			return false
		}
	}
	return true
}

func contains(slice [][]byte, target []byte) bool {
	for _, element := range slice {
		if equal(element, target) {
			return true
		}
	}
	return false
}

func Subslice(smaller [][]byte, bigger [][]byte) bool {
	for _, elem := range smaller {
		if !contains(bigger, elem) {
			return false
		}
	}
	return true
}

func EqualSlices(x [][]byte, y [][]byte) bool {
	return Subslice(x, y) && Subslice(y, x)
}

func EmptyIntersection(x [][]byte, y [][]byte) bool {
	for _, elem := range x {
		if contains(y, elem) {
			return false
		}
	}
	return true
}

func SliceDiff(bigger [][]byte, smaller [][]byte) [][]byte {
	var diff [][]byte
	for _, elem := range bigger {
		if !contains(smaller, elem) {
			diff = append(diff, elem)
		}
	}
	return diff
}
