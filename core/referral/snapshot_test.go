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

package referral_test

import (
	"bytes"
	"testing"
	"time"

	"code.vegaprotocol.io/vega/core/types"
	"code.vegaprotocol.io/vega/libs/num"
	vgtest "code.vegaprotocol.io/vega/libs/test"
	"code.vegaprotocol.io/vega/paths"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTakingAndRestoringSnapshotSucceeds(t *testing.T) {
	ctx := vgtest.VegaContext("chainid", 100)

	vegaPath := paths.New(t.TempDir())
	now := time.Now()
	maxVolumeParams := num.UintFromUint64(100)

	te1 := newEngine(t)
	snapshotEngine1 := newSnapshotEngine(t, vegaPath, now, te1.engine)
	closeSnapshotEngine1 := vgtest.OnlyOnce(snapshotEngine1.Close)
	defer closeSnapshotEngine1()

	require.NoError(t, snapshotEngine1.Start(ctx))

	// Cap the notional volume.
	require.NoError(t, te1.engine.OnReferralProgramMaxPartyNotionalVolumeByQuantumPerEpochUpdate(ctx, maxVolumeParams))
	require.NoError(t, te1.engine.OnReferralProgramMinStakedVegaTokensUpdate(ctx, num.NewUint(100)))

	referrer1 := newPartyID(t)
	referrer2 := newPartyID(t)
	referrer3 := newPartyID(t)
	referrer4 := newPartyID(t)
	referee1 := newPartyID(t)
	referee2 := newPartyID(t)
	referee3 := newPartyID(t)
	referee4 := newPartyID(t)
	referee5 := newPartyID(t)
	referee6 := newPartyID(t)
	referee7 := newPartyID(t)
	referee8 := newPartyID(t)
	referee9 := newPartyID(t)

	te1.broker.EXPECT().Send(gomock.Any()).Times(13)
	te1.timeSvc.EXPECT().GetTimeNow().Return(now).Times(13)
	te1.staking.EXPECT().GetAvailableBalance(gomock.Any()).AnyTimes().Return(num.NewUint(100), nil)

	assert.NoError(t, te1.engine.CreateReferralSet(ctx, referrer1, "id1"))
	assert.NoError(t, te1.engine.CreateReferralSet(ctx, referrer2, "id2"))
	assert.NoError(t, te1.engine.CreateReferralSet(ctx, referrer3, "id3"))
	assert.NoError(t, te1.engine.CreateReferralSet(ctx, referrer4, "id4"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee1, "id1"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee2, "id4"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee3, "id3"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee4, "id2"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee5, "id2"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee6, "id2"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee7, "id1"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee8, "id4"))
	assert.NoError(t, te1.engine.ApplyReferralCode(ctx, referee9, "id3"))

	program1 := &types.ReferralProgram{
		EndOfProgramTimestamp: now.Add(24 * time.Hour),
		WindowLength:          10,
		BenefitTiers:          []*types.BenefitTier{},
		StakingTiers:          []*types.StakingTier{},
	}

	te1.engine.UpdateProgram(program1)

	// Simulating end of epoch.
	// The program should be applied.
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer1)).Return(num.UintFromUint64(10)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer2)).Return(num.UintFromUint64(20)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer3)).Return(num.UintFromUint64(30)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer4)).Return(num.UintFromUint64(40)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee1)).Return(num.UintFromUint64(50)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee2)).Return(num.UintFromUint64(60)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee3)).Return(num.UintFromUint64(70)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee4)).Return(num.UintFromUint64(80)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee5)).Return(num.UintFromUint64(90)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee6)).Return(num.UintFromUint64(100)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee7)).Return(num.UintFromUint64(110)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee8)).Return(num.UintFromUint64(120)).Times(1)
	te1.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee9)).Return(num.UintFromUint64(130)).Times(1)

	expectReferralProgramStartedEvent(t, te1)
	lastEpochStartTime := program1.EndOfProgramTimestamp.Add(-2 * time.Hour)
	nextEpoch(t, ctx, te1, lastEpochStartTime)

	program2 := &types.ReferralProgram{
		EndOfProgramTimestamp: lastEpochStartTime.Add(10 * time.Hour),
		WindowLength:          10,
		BenefitTiers:          []*types.BenefitTier{},
	}

	// Set new program.
	te1.engine.UpdateProgram(program2)

	// Simulating end of epoch.
	// The program should be updated with the new one.
	postSnapshotActions := func(te *testEngine) {
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer1)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer2)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer3)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer4)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee1)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee2)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee3)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee4)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee5)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee6)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee7)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee8)).Return(num.UintFromUint64(100)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee9)).Return(num.UintFromUint64(100)).Times(1)
		te.staking.EXPECT().GetAvailableBalance(gomock.Any()).AnyTimes().Return(num.NewUint(100), nil)

		gomock.InOrder(
			expectReferralSetStatsUpdatedEvent(t, te, 4),
			expectReferralProgramUpdatedEvent(t, te),
		)
		lastEpochStartTime = program2.EndOfProgramTimestamp.Add(-2 * time.Hour)
		nextEpoch(t, ctx, te, lastEpochStartTime)

		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer1)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer2)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer3)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referrer4)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee1)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee2)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee3)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee4)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee5)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee6)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee7)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee8)).Return(num.UintFromUint64(200)).Times(1)
		te.marketActivityTracker.EXPECT().NotionalTakerVolumeForParty(string(referee9)).Return(num.UintFromUint64(200)).Times(1)

		expectReferralSetStatsUpdatedEvent(t, te, 4)
		lastEpochStartTime = program2.EndOfProgramTimestamp.Add(-1 * time.Hour)
		nextEpoch(t, ctx, te, lastEpochStartTime)
	}
	postSnapshotActions(te1)

	// Take a snapshot.
	hash1, err := snapshotEngine1.SnapshotNow(ctx)
	require.NoError(t, err)

	closeSnapshotEngine1()

	// Reload the engine using the previous snapshot.
	te2 := newEngine(t)
	snapshotEngine2 := newSnapshotEngine(t, vegaPath, now, te2.engine)
	defer snapshotEngine2.Close()

	// Simulate restoration of the network parameter at the time of the snapshot
	require.NoError(t, te2.engine.OnReferralProgramMaxPartyNotionalVolumeByQuantumPerEpochUpdate(ctx, maxVolumeParams))

	// This triggers the state restoration from the local snapshot.
	require.NoError(t, snapshotEngine2.Start(ctx))

	// Comparing the hash after restoration, to ensure it produces the same result.
	hash2, _, _ := snapshotEngine2.Info()
	require.True(t, bytes.Equal(hash1, hash2))
}
