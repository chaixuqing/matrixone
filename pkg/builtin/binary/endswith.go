// Copyright 2022 Matrix Origin
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

package binary

import (
	"errors"
	"fmt"

	"github.com/matrixorigin/matrixone/pkg/builtin"
	"github.com/matrixorigin/matrixone/pkg/container/nulls"
	"github.com/matrixorigin/matrixone/pkg/container/types"
	"github.com/matrixorigin/matrixone/pkg/container/vector"
	"github.com/matrixorigin/matrixone/pkg/encoding"
	"github.com/matrixorigin/matrixone/pkg/sql/colexec/extend"
	"github.com/matrixorigin/matrixone/pkg/sql/colexec/extend/overload"
	"github.com/matrixorigin/matrixone/pkg/vectorize/endswith"
	"github.com/matrixorigin/matrixone/pkg/vm/process"
)

var argAndRets = []argsAndRet{
	{[]types.T{types.T_char}, types.T_uint8},
	{[]types.T{types.T_varchar}, types.T_uint8},
}

func init() {
	fmt.Println("endswith init")
	extend.FunctionRegistry["endswith"] = builtin.EndsWith
	for _, item := range argAndRets {
		overload.AppendFunctionRets(builtin.EndsWith, item.args, item.ret)
	}
	extend.BinaryReturnTypes[builtin.EndsWith] = func(e extend.Extend, e2 extend.Extend) types.T {
		return types.T_uint8
	}

	extend.BinaryStrings[builtin.EndsWith] = func(e extend.Extend, e2 extend.Extend) string {
		return fmt.Sprintf("endswith(%s, %s)", e, e2)
	}

	overload.OpTypes[builtin.EndsWith] = overload.Binary

	overload.BinOps[builtin.EndsWith] = []*overload.BinOp{
		{
			LeftType:   types.T_varchar,
			RightType:  types.T_varchar,
			ReturnType: types.T_uint8,
			Fn: func(lv, rv *vector.Vector, proc *process.Process, lc, rc bool) (*vector.Vector, error) {
				lvs, rvs := lv.Col.(*types.Bytes), rv.Col.(*types.Bytes)
				if !lc && !rc {
					return nil, errors.New("endswith() needs two arguments")
				}
				fmt.Println("int64(len(lvs.Data))=", int64(len(lvs.Data)))
				resultVector, err := process.Get(proc, int64(len(lvs.Data)), types.Type{Oid: types.T_uint8, Size: 1})

				if err != nil {
					return nil, err
				}
				results := encoding.DecodeUint8Slice(resultVector.Data)
				results = results[:len(lvs.Data)]
				resultVector.Col = results
				nulls.Or(lv.Nsp, rv.Nsp, resultVector.Nsp)
				fmt.Println("results length=", len(results))
				vector.SetCol(resultVector, endswith.EndsWith(lvs, rvs, results))
				return resultVector, nil
			},
		},
		{
			LeftType:   types.T_char,
			RightType:  types.T_char,
			ReturnType: types.T_uint8,
			Fn: func(lv, rv *vector.Vector, proc *process.Process, lc, rc bool) (*vector.Vector, error) {
				lvs, rvs := lv.Col.(*types.Bytes), rv.Col.(*types.Bytes)
				if !lc && !rc {
					return nil, errors.New("endswith() needs two arguments")
				}

				resultVector, err := process.Get(proc, int64(len(lvs.Data)), types.Type{Oid: types.T_uint8, Size: 1})

				if err != nil {
					return nil, err
				}
				results := encoding.DecodeUint8Slice(resultVector.Data)
				results = results[:len(lvs.Data)]
				resultVector.Col = results
				nulls.Or(lv.Nsp, rv.Nsp, resultVector.Nsp)
				vector.SetCol(resultVector, endswith.EndsWith(lvs, rvs, results))
				return resultVector, nil
			},
		},
	}
}
