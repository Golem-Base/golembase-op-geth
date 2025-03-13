// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
)

// GolemBaseHousekeepingTx is the transaction data of the original Ethereum transactions.
type GolemBaseHousekeepingTx struct {
	Nonce    uint64          // nonce of sender account
	GasPrice *big.Int        // wei per gas
	Gas      uint64          // gas limit
	To       *common.Address `rlp:"nil"` // nil means contract creation
	Value    *big.Int        // wei amount
	Data     []byte          // contract invocation input data
	V, R, S  *big.Int        // signature values
}

// NewGolemBaseHousekeepingTransaction creates an unsigned golem base housekeeping transaction.
// Deprecated: use NewTx instead.
func NewGolemBaseHousekeepingTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return NewTx(&GolemBaseHousekeepingTx{
		Nonce:    nonce,
		To:       &to,
		Value:    amount,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	})
}

// copy creates a deep copy of the transaction data and initializes all fields.
func (tx *GolemBaseHousekeepingTx) copy() TxData {
	cpy := &GolemBaseHousekeepingTx{
		Nonce: tx.Nonce,
		To:    copyAddressPtr(tx.To),
		Data:  common.CopyBytes(tx.Data),
		Gas:   tx.Gas,
		// These are initialized below.
		Value:    new(big.Int),
		GasPrice: new(big.Int),
		V:        new(big.Int),
		R:        new(big.Int),
		S:        new(big.Int),
	}
	if tx.Value != nil {
		cpy.Value.Set(tx.Value)
	}
	if tx.GasPrice != nil {
		cpy.GasPrice.Set(tx.GasPrice)
	}
	if tx.V != nil {
		cpy.V.Set(tx.V)
	}
	if tx.R != nil {
		cpy.R.Set(tx.R)
	}
	if tx.S != nil {
		cpy.S.Set(tx.S)
	}
	return cpy
}

// accessors for innerTx.
func (tx *GolemBaseHousekeepingTx) txType() byte           { return GolemBaseHousekeepingTxType }
func (tx *GolemBaseHousekeepingTx) chainID() *big.Int      { return deriveChainId(tx.V) }
func (tx *GolemBaseHousekeepingTx) accessList() AccessList { return nil }
func (tx *GolemBaseHousekeepingTx) data() []byte           { return tx.Data }
func (tx *GolemBaseHousekeepingTx) gas() uint64            { return tx.Gas }
func (tx *GolemBaseHousekeepingTx) gasPrice() *big.Int     { return tx.GasPrice }
func (tx *GolemBaseHousekeepingTx) gasTipCap() *big.Int    { return tx.GasPrice }
func (tx *GolemBaseHousekeepingTx) gasFeeCap() *big.Int    { return tx.GasPrice }
func (tx *GolemBaseHousekeepingTx) value() *big.Int        { return tx.Value }
func (tx *GolemBaseHousekeepingTx) nonce() uint64          { return tx.Nonce }
func (tx *GolemBaseHousekeepingTx) to() *common.Address    { return tx.To }
func (tx *GolemBaseHousekeepingTx) isSystemTx() bool       { return false }

func (tx *GolemBaseHousekeepingTx) effectiveGasPrice(dst *big.Int, baseFee *big.Int) *big.Int {
	return dst.Set(tx.GasPrice)
}

func (tx *GolemBaseHousekeepingTx) rawSignatureValues() (v, r, s *big.Int) {
	return tx.V, tx.R, tx.S
}

func (tx *GolemBaseHousekeepingTx) setSignatureValues(chainID, v, r, s *big.Int) {
	tx.V, tx.R, tx.S = v, r, s
}

func (tx *GolemBaseHousekeepingTx) encode(b *bytes.Buffer) error {
	return rlp.Encode(b, tx)
}

func (tx *GolemBaseHousekeepingTx) decode(input []byte) error {
	return rlp.DecodeBytes(input, tx)
}
