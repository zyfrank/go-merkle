/* Copyright 2013 Steve Leonard <sleonard76@gmail.com>. All rights reserved.
Use of this source code is governed by the MIT license that can be found
in the LICENSE file.
*/

/* Package merkle is a fixed merkle tree implementation */
package merkle

import (
	"bytes"
	"errors"
	"fmt"
	"hash"
)

// TreeOptions configures tree behavior
type SMTOptions struct {
	// EnableHashSorting modifies the tree's hash behavior to sort the hashes before concatenating them
	// to calculate the parent hash. This removes the capability of proving the position in the tree but
	// simplifies the proof format by removing the need to specify left/right.
	EnableHashSorting bool

	// DisableHashLeaves determines whether leaf nodes should be hashed or not. By doing disabling this behavior,
	// you can use a different hash function for leaves or generate a tree that contains already hashed
	// values.
	DisableHashLeaves bool
}

// Node in the merkle tree
type SMTNode struct {
	Hash  []byte
	Left  *Node
	Right *Node
}

// Tree contains all nodes
type SMT struct {
	// All nodes, linear
	//Nodes []Node
	// Points to each level in the node. The first level contains the root node
	//Levels [][]Node
	// Any particular behavior changing option
	Options SMTOptions

	// If left and right child hash of one node are same, then cache this node's hash
	//NonLeafCachedHashes map[string][]byte
	EmptyLeafHash []byte

  CachedAllLevelsHashOfEmptyLeaves [][]byte

  Root []byte
}

func NewSMTWithOpts(options SMTOptions) SMT {
	tree := NewSMT()
	tree.Options = options
	return tree
}

func NewSMT() SMT {
	return SMT{}
}

func (self *SMT) Generate(blocks [][]byte, hashf hash.Hash) error {
	return self.GenerateByTwoHashFunc(blocks, hashf, hashf)
}

func emptyNodeHash(h hash.Hash) ([]byte, error) {
	defer h.Reset()
	_, err := h.Write([]byte{})
	if err != nil {
		return []byte{}, err
	}
	hash := h.Sum(nil)
	return hash, nil
}

// Generates the tree nodes by using different hash funtions between internal and leaf node
func (self *SMT) GenerateByTwoHashFunc(blocks [][]byte, leafHash hash.Hash, nonLeafHash hash.Hash) error {
  if !isPowerOfTwo(uint64(len(blocks))) {
		return errors.New("Leaves number of SMT tree should be power of 2")
	}
  root, err := self.GenerateSMT(0, len(blocks)-1, blocks, leafHash, nonLeafHash)
  fmt.Printf("root is    %v\n\n", root)
  if err == nil {
    self.Root = root
  }
	return err
}

func (self *SMT) GetRoot() []byte {
  return self.Root
}

func computeHashByTwoItems(item1 []byte, item2 []byte, hash hash.Hash) ([]byte, error) {
	defer hash.Reset()
	combinedItem := make([]byte, len(item1)+len(item2))
	copy(combinedItem[:len(item1)], item1)
	copy(combinedItem[len(item1):], item2)

	_, err := hash.Write(combinedItem[:])
	if err != nil {
		return []byte{}, err
  }
	return hash.Sum(nil), nil
}

func (self *SMT) computeHashOfOneItem(item []byte, hash hash.Hash) ([]byte, error) {
  defer hash.Reset()
	if bytes.Compare(item, []byte{}) == 0 {
		var result []byte
		var err error
		if self.EmptyLeafHash != nil {
			result = self.EmptyLeafHash
		} else {
			result, err = emptyNodeHash(hash)
			if err != nil {
				return []byte{}, err
			}
			self.EmptyLeafHash = result
		}
		return result, nil
	}
	_, err := hash.Write(item[:])
	if err != nil {
		return []byte{}, err
	}
	return hash.Sum(nil), nil
}

func (self *SMT) ComputeEmptyLeavesSubTreeHash(leavesNumber int, leafHash hash.Hash, nonLeafHash hash.Hash) ([]byte, error) {
	//fmt.Printf("leaves numer is %d\n", leavesNumber)
	if 2 == leavesNumber {
		var hash []byte
		var err error
		if self.EmptyLeafHash != nil && !bytes.Equal(self.EmptyLeafHash, []byte{}){
      hash = self.EmptyLeafHash
		} else {
      hash, err = emptyNodeHash(leafHash)
			if err != nil {
				return []byte{}, err
			}
			self.EmptyLeafHash = hash
    }
		if self.CachedAllLevelsHashOfEmptyLeaves != nil && len(self.CachedAllLevelsHashOfEmptyLeaves) > 0 {
      hash = self.CachedAllLevelsHashOfEmptyLeaves[0]
		} else {
      hash, err = computeHashByTwoItems(hash, hash, nonLeafHash)
			if err != nil {
				return []byte{}, err
      }
      self.CachedAllLevelsHashOfEmptyLeaves = append(self.CachedAllLevelsHashOfEmptyLeaves, hash)
		}
		return hash, nil
	}

	levels := logBaseTwo(uint64(leavesNumber)) - 1
	if self.CachedAllLevelsHashOfEmptyLeaves != nil && uint64(len(self.CachedAllLevelsHashOfEmptyLeaves)) > levels {
		return self.CachedAllLevelsHashOfEmptyLeaves[levels], nil
	}

	nextLevelHash, err := self.ComputeEmptyLeavesSubTreeHash(leavesNumber/2, leafHash, nonLeafHash)
	if err != nil {
		return []byte{}, nil
	}
  combinedHash, err := computeHashByTwoItems(nextLevelHash, nextLevelHash, nonLeafHash)
	self.CachedAllLevelsHashOfEmptyLeaves = append(self.CachedAllLevelsHashOfEmptyLeaves, combinedHash)

	return combinedHash, nil
}

func (self *SMT) GenerateSMT(start int, end int, blocks [][]byte, leafHash hash.Hash, nonLeafHash hash.Hash) ([]byte, error) {
  //fmt.Println("here called")
	totalEle := (end - start) + 1

  if bytes.Compare(blocks[start], []byte{}) == 0 {
		return self.ComputeEmptyLeavesSubTreeHash(totalEle, leafHash, nonLeafHash)
	}

	if totalEle == 2 {
  	left, err := self.computeHashOfOneItem(blocks[start], leafHash)
		if err != nil {
			return []byte{}, nil
		}
    right, err := self.computeHashOfOneItem(blocks[start+1], leafHash)
    if err != nil {
			return []byte{}, nil
    }
  	return computeHashByTwoItems(left, right, nonLeafHash)
	}

	leftStart := start
	leftEnd := start + (totalEle / 2) - 1
	rightStart := leftEnd + 1
	rightEnd := end

  var rightHash []byte
  var err error
	if bytes.Compare(blocks[rightStart], []byte{}) == 0 {
    rightHash, err = self.ComputeEmptyLeavesSubTreeHash(rightEnd-rightStart+1, leafHash, nonLeafHash)
	} else {
    rightHash, err = self.GenerateSMT(rightStart, rightEnd, blocks, leafHash, nonLeafHash)
	}
	if err != nil {
		return []byte{}, err
	}

	leftHash, err := self.GenerateSMT(leftStart, leftEnd, blocks, leafHash, nonLeafHash)
	if err != nil {
		return []byte{}, err
  }
	return computeHashByTwoItems(leftHash, rightHash, nonLeafHash)

}
