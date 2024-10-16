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

package future_test

import (
	"context"
	"testing"
	"time"

	"code.vegaprotocol.io/vega/core/events"
	"code.vegaprotocol.io/vega/core/types"
	vegacontext "code.vegaprotocol.io/vega/libs/context"
	vgcrypto "code.vegaprotocol.io/vega/libs/crypto"
	"code.vegaprotocol.io/vega/libs/num"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type equityShareMarket struct {
	t       *testing.T
	tm      *testMarket
	parties map[string]struct{}

	Now       time.Time
	ClosingAt time.Time
}

func newEquityShareMarket(t *testing.T) *equityShareMarket {
	t.Helper()
	now := time.Unix(10, 0)
	closingAt := time.Unix(10000000000, 0)

	return &equityShareMarket{
		t:         t,
		tm:        getTestMarket(t, now, nil, &types.AuctionDuration{Duration: 1}),
		parties:   map[string]struct{}{},
		Now:       now,
		ClosingAt: closingAt,
	}
}

func (esm *equityShareMarket) TestMarket() *testMarket { return esm.tm }

func (esm *equityShareMarket) BuildOrder(id, party string, side types.Side, price uint64) *types.Order {
	return &types.Order{
		Type:        types.OrderTypeLimit,
		TimeInForce: types.OrderTimeInForceGTT,
		Status:      types.OrderStatusActive,
		ID:          id,
		Side:        side,
		Party:       party,
		MarketID:    esm.tm.market.GetID(),
		Size:        1,
		Price:       num.NewUint(price),
		Remaining:   1,
		CreatedAt:   esm.Now.UnixNano(),
		ExpiresAt:   esm.ClosingAt.UnixNano(),
	}
}

func (esm *equityShareMarket) createPartyIfMissing(t *testing.T, party string) {
	t.Helper()
	if _, ok := esm.parties[party]; !ok {
		esm.parties[party] = struct{}{}
		addAccount(t, esm.tm, party)
	}
}

func (esm *equityShareMarket) SubmitOrder(t *testing.T, ctx context.Context, order *types.Order) (*types.OrderConfirmation, error) {
	t.Helper()
	esm.createPartyIfMissing(t, order.Party)
	return esm.tm.market.SubmitOrder(ctx, order)
}

func (esm *equityShareMarket) WithSubmittedOrder(t *testing.T, id, party string, side types.Side, price uint64) *equityShareMarket {
	t.Helper()
	ctx := context.Background()
	order := esm.BuildOrder(id, party, side, price)

	_, err := esm.SubmitOrder(t, ctx, order)
	require.NoError(esm.t, err)
	return esm
}

func (esm *equityShareMarket) WithSubmittedLiquidityProvision(t *testing.T, party, id string, amount uint64, fee string) *equityShareMarket {
	t.Helper()
	esm.createPartyIfMissing(t, party)
	esm.tm.WithSubmittedLiquidityProvision(esm.t, party, amount, fee)
	return esm
}

func (esm *equityShareMarket) LiquidityFeeAccount() *types.Account {
	acc, err := esm.tm.collateralEngine.GetMarketLiquidityFeeAccount(
		esm.tm.market.GetID(), esm.tm.asset,
	)
	require.NoError(esm.t, err)
	return acc
}

func (esm *equityShareMarket) PartyGeneralAccount(party string) *types.Account {
	acc, err := esm.tm.collateralEngine.GetPartyGeneralAccount(
		party, esm.tm.asset,
	)
	require.NoError(esm.t, err)
	return acc
}

func (esm *equityShareMarket) PartyLiquidityFeeAccount(party string) *types.Account {
	acc, err := esm.tm.collateralEngine.GetPartyLiquidityFeeAccount(
		esm.tm.market.GetID(), party, esm.tm.asset,
	)
	require.NoError(esm.t, err)
	return acc
}

func (esm *equityShareMarket) PartyMarginAccount(party string) *types.Account {
	acc, err := esm.tm.collateralEngine.GetPartyMarginAccount(
		esm.tm.market.GetID(), party, esm.tm.asset,
	)
	require.NoError(esm.t, err)
	return acc
}

func TestWithinMarket(t *testing.T) {
	var (
		ctx = vegacontext.WithTraceID(context.Background(), vgcrypto.RandomHash())
		// as we will split fees in 1/3 and 2/3
		// we use 900000 cause we need this number to be divisible by 3
		matchingPrice = uint64(900000)
		one           = uint64(1)
	)

	// Setup a market with a set of non-matching orders and Liquidity Provision
	// Submissions from 2 parties.
	esm := newEquityShareMarket(t).
		WithSubmittedOrder(t, "some-id-1", "party1", types.SideSell, matchingPrice+one).
		WithSubmittedOrder(t, "some-id-2", "party2", types.SideBuy, matchingPrice-one).
		WithSubmittedOrder(t, "some-id-3", "party1", types.SideSell, matchingPrice).
		WithSubmittedOrder(t, "some-id-4", "party2", types.SideBuy, matchingPrice). // Need to generate a trade to leave opening auction
		// party1 (commitment: 2000) should get 2/3 of the fee
		WithSubmittedLiquidityProvision(t, "party1", "lp-id-1", 2000000, "0.5").
		// party2 (commitment: 1000) should get 1/3 of the fee
		WithSubmittedLiquidityProvision(t, "party2", "lp-id-2", 1000000, "0.5")

	// tm is the testMarket instance
	var (
		tm      = esm.TestMarket()
		curTime = esm.Now
	)

	// End opening auction
	curTime = curTime.Add(2 * time.Second)
	tm.now = curTime
	tm.market.OnTick(ctx, curTime)

	md := esm.tm.market.GetMarketData()
	require.NotNil(t, md)
	require.Equal(t, types.MarketTradingModeContinuous, md.MarketTradingMode)

	t.Run("WhenNoTrades", func(t *testing.T) {
		// clean up previous events
		tm.events = []events.Event{}

		// Trigger Fee distribution
		curTime = curTime.Add(1 * time.Second)
		tm.now = curTime
		tm.market.OnTick(ctx, curTime)

		// Assert the event
		var evt *events.LedgerMovements
		for _, e := range tm.events {
			if e.Type() == events.LedgerMovementsEvent {
				evt = e.(*events.LedgerMovements)
			}
		}
		require.Nil(t, evt, "should receive no TransferEvent")
	})

	// Match a pair of orders (same price) to trigger a fee distribution.
	conf, err := esm.
		WithSubmittedOrder(t, "some-id-3", "party1", types.SideSell, matchingPrice).
		SubmitOrder(t, context.Background(), esm.BuildOrder("some-id-4", "party2", types.SideBuy, matchingPrice))
	require.NoError(t, err)
	require.Len(t, conf.Trades, 1)

	// Retrieve both MarketLiquidityFee account balance and Party Balance
	// before the fee distribution.

	originalBalance := esm.LiquidityFeeAccount().Balance.Clone()

	curTime = curTime.Add(1 * time.Second)
	tm.now = curTime
	tm.market.OnTick(ctx, curTime)

	md = esm.tm.market.GetMarketData()
	require.NotNil(t, md)
	require.Equal(t, types.MarketTradingModeContinuous, md.MarketTradingMode)

	oneU := num.NewUint(1)
	assert.True(t, esm.LiquidityFeeAccount().Balance.EQ(oneU),
		"LiquidityFeeAccount should have a balance of 1 (remainder)")

	// exp = originalBalance*(2/3)
	exp := num.UintZero().Mul(num.Sum(oneU, oneU), originalBalance)
	exp = exp.Div(exp, num.Sum(oneU, oneU, oneU))
	actual := esm.PartyLiquidityFeeAccount("party1").Balance
	assert.True(t,
		exp.EQ(actual),
		"party1 should get 2/3 of the fees (got %s expected %s)", actual.String(), exp.String(),
	)

	// exp = originalBalance*(1/3)
	exp = num.UintZero().Div(originalBalance, num.Sum(oneU, oneU, oneU))
	// minus the remainder
	exp.Sub(exp, oneU)
	actual = esm.PartyLiquidityFeeAccount("party2").Balance
	assert.True(t,
		exp.EQ(actual),
		"party2 should get 2/3 of the fees (got %s expected %s)", actual.String(), exp.String(),
	)
}
