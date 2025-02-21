// Copyright 2021 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vm

const (
	Top = iota
	Join
	Plus
	Limit
	Merge
	Dedup
	Order
	Oplus
	Output
	Offset
	Restrict
	Connector
	Transform
	Projection
	UnTransform

	MergeDedup
	MergeLimit
	MergeOffset
	MergeOrder
	MergeTop

	DeleteTag
	UpdateTag
)

// Instruction contains relational algebra
type Instruction struct {
	// Op specified the operator code of an instruction.
	Op int
	// Arg contains the operand of this instruction.
	Arg interface{}
}

type Instructions []Instruction
