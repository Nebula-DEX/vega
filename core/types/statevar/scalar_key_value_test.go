// Copyright (C) 2023 Gobalsky Labs Limited
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package statevar_test

import (
	"testing"

	"code.vegaprotocol.io/vega/core/types/statevar"
	"code.vegaprotocol.io/vega/libs/num"
	"code.vegaprotocol.io/vega/protos/vega"

	"github.com/stretchr/testify/require"
)

func TestDecimalScalar(t *testing.T) {
	t.Run("test equality of two decimal scalars", testDecimalScalarEquality)
	t.Run("test two scalar decimals are within tolerance of each other", testScalarWithinTol)
	t.Run("test converion of decimal scalar to a decimal scalar", testScalarToDecimal)
	t.Run("test conversion to proto", testScalarToProto)
}

// testDecimalScalarEquality tests that given the same key and equal/not equal value, equals function returns the correct value.
func testDecimalScalarEquality(t *testing.T) {
	kvb1 := &statevar.KeyValueBundle{}
	kvb1.KVT = append(kvb1.KVT, statevar.KeyValueTol{
		Key: "scalar value",
		Val: &statevar.DecimalScalar{Val: num.DecimalFromFloat(1.23456)},
	})

	kvb2 := &statevar.KeyValueBundle{}
	kvb2.KVT = append(kvb2.KVT, statevar.KeyValueTol{
		Key: "scalar value",
		Val: &statevar.DecimalScalar{Val: num.DecimalFromFloat(6.54321)},
	})

	require.False(t, kvb1.Equals(kvb2))

	kvb3 := &statevar.KeyValueBundle{}
	kvb3.KVT = append(kvb3.KVT, statevar.KeyValueTol{
		Key: "scalar value",
		Val: &statevar.DecimalScalar{Val: num.DecimalFromFloat(1.23456)},
	})
	require.True(t, kvb1.Equals(kvb3))
}

func testScalarWithinTol(t *testing.T) {
	kvb1 := &statevar.KeyValueBundle{}
	kvb1.KVT = append(kvb1.KVT, statevar.KeyValueTol{
		Key:       "scalar value",
		Val:       &statevar.DecimalScalar{Val: num.DecimalFromFloat(1.23456)},
		Tolerance: num.DecimalFromInt64(1),
	})

	kvb2 := &statevar.KeyValueBundle{}
	kvb2.KVT = append(kvb2.KVT, statevar.KeyValueTol{
		Key:       "scalar value",
		Val:       &statevar.DecimalScalar{Val: num.DecimalFromFloat(6.54321)},
		Tolerance: num.DecimalFromInt64(1),
	})

	require.False(t, kvb1.WithinTolerance(kvb2))

	kvb3 := &statevar.KeyValueBundle{}
	kvb3.KVT = append(kvb3.KVT, statevar.KeyValueTol{
		Key:       "scalar value",
		Val:       &statevar.DecimalScalar{Val: num.DecimalFromFloat(2.23456)},
		Tolerance: num.DecimalFromInt64(1),
	})
	require.True(t, kvb1.WithinTolerance(kvb3))
}

// testScalarToDecimal tests conversion to decimal.
func testScalarToDecimal(t *testing.T) {
	kvb1 := &statevar.KeyValueBundle{}
	kvb1.KVT = append(kvb1.KVT, statevar.KeyValueTol{
		Key:       "scalar value",
		Val:       &statevar.DecimalScalar{Val: num.DecimalFromFloat(1.23456)},
		Tolerance: num.DecimalFromInt64(1),
	})

	res := kvb1.KVT[0].Val
	switch v := res.(type) {
	case *statevar.DecimalScalar:
		require.Equal(t, num.DecimalFromFloat(1.23456), v.Val)
	default:
		t.Fail()
	}
}

func testScalarToProto(t *testing.T) {
	kvb1 := &statevar.KeyValueBundle{}
	kvb1.KVT = append(kvb1.KVT, statevar.KeyValueTol{
		Key:       "scalar value",
		Val:       &statevar.DecimalScalar{Val: num.DecimalFromFloat(1.23456)},
		Tolerance: num.DecimalFromInt64(1),
	})
	res := kvb1.ToProto()
	require.Equal(t, 1, len(res))
	require.Equal(t, "scalar value", res[0].Key)
	require.Equal(t, "1", res[0].Tolerance)
	switch v := res[0].Value.Value.(type) {
	case *vega.StateVarValue_ScalarVal:
		require.Equal(t, "1.23456", v.ScalarVal.Value)
	default:
		t.Fail()
	}

	kvb2, err := statevar.KeyValueBundleFromProto(res)
	require.NoError(t, err)
	require.Equal(t, kvb1, kvb2)
}
