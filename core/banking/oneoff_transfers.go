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

package banking

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"code.vegaprotocol.io/vega/core/events"
	"code.vegaprotocol.io/vega/core/types"
	vgcontext "code.vegaprotocol.io/vega/libs/context"
	"code.vegaprotocol.io/vega/libs/crypto"
	"code.vegaprotocol.io/vega/logging"
	checkpoint "code.vegaprotocol.io/vega/protos/vega/checkpoint/v1"
)

var (
	ErrUnsupportedTransferKind         = errors.New("unsupported transfer kind")
	ErrNebTransfersTemporarilyDisabled = errors.New("neb transfers temporarily disabled")
	nebTransferAllowList               = map[string]struct{}{
		"6da64ee751d722a4ce32f674ac5502825273162bc53ebcbc42356d8a840736d6": {},
		"060644ae17d07dc44d71bb9f360f8b78ad371a5875115e5998ea56cf71b066d9": {},
		"4d6f10f38991119238945d685a7799881cc67b41a06c1bb1dec9d172c332a980": {},
		"1d544d84277d2a35bcee57034bc17642383d4114a78582f3fc0fc989c08f2bca": {},
		"d96b5a2f5b008bc4c786f819c5cf28686345fead26d874030e1189fbb7372675": {},
		"f018c7eeec2cf1270df64f669edee2a6b5113ad42cdd888abda311363ad52255": {},
		"3a657a9ff5ebf6f85bd4f86047c73cac5e0d3d5c8079bcba88b175e7df935d4d": {},
		"36cf40695b026419e18da8e49bcaac4aedb2137330f08bb3735fd7a016085611": {},
		"0cf08b62b1815666e6628bb9d62e9361a3ee79131d64dc36966e4a414232b4b4": {},
		"47bbe613b6bcf21c9139675115b23c88bbef246ee319c961e9b2ae63a83fea4e": {},
		"fbdbcbfbb47d67b6a98422f6d750916d7bab571c2432c24bdbd6d2c97bbcb14c": {},
		"2a184dcc53b77085b35002bf193e664273f79765b1add50f1251beddf7d799a4": {},
		"a614792954895c9dcd05d1f047ae726be380f25ef3f732cc2207b84b2b67bf2b": {},
		"875a5f31c9ae977e8f42fd629a0004d83019d177925ed748ec59f9c01722d5b4": {},
		"dde3a4ca0eb0ef93ac892062f1f74c43a52234a8be13c4504989365a09029e15": {},
		"54d3289644c6ea74ad456c38276e5a6a2a64e7de93273a081bb1cb4169dfee47": {},
		"60ab1e9ea4135ec9f68029a0f29c20e318a985f57f59eb5fc50033d625789d35": {},
		"7fea1b8650570f6d09fac7b020490e6505718f32933b049cb691b3eddc7c43ad": {},
		"1443063174d0e760e4835299e67ad2e5c2309bcb692d89f71e6967abbf60e8cc": {},
		"16332f04db4f7451198b960991e1d76ee979188f06f726ce90b960e12f1c787f": {},
		"18195f59b342617db0b41cb6489a95b4061a56138aadb4cbd65f2d1c0d1dd8da": {},
		"0f38d2f260cef10036d30e64df818a07bc34f08263bd65617b4cd2900760d20e": {},
		"8649926e708b46a04401a97abdce7d7f1ba12e233330ef00331d83c758911a97": {},
		"6a9e18ebd0775077ffaa473e74cf4f2cff266a1ec53041083474451976d3fd97": {},
		"afd48ac22156f2f3860dcb56b64ca60f72c9346b7fe8596a96079998eb1319b5": {},
		"5dee590230a5497a39dc7a7d7adeb0de8816403c4185168f08920c3c5180ea21": {},
	}
)

type scheduledTransfer struct {
	// to send events
	oneoff      *types.OneOffTransfer
	transfer    *types.Transfer
	accountType types.AccountType
	reference   string
}

func (s *scheduledTransfer) ToProto() *checkpoint.ScheduledTransfer {
	return &checkpoint.ScheduledTransfer{
		OneoffTransfer: s.oneoff.IntoEvent(nil),
		Transfer:       s.transfer.IntoProto(),
		AccountType:    s.accountType,
		Reference:      s.reference,
	}
}

func scheduledTransferFromProto(p *checkpoint.ScheduledTransfer) (scheduledTransfer, error) {
	transfer, err := types.TransferFromProto(p.Transfer)
	if err != nil {
		return scheduledTransfer{}, err
	}

	return scheduledTransfer{
		oneoff:      types.OneOffTransferFromEvent(p.OneoffTransfer),
		transfer:    transfer,
		accountType: p.AccountType,
		reference:   p.Reference,
	}, nil
}

func (e *Engine) updateStakingAccounts(
	ctx context.Context, transfer *types.OneOffTransfer,
) {
	if transfer.Asset != e.stakingAsset {
		// nothing to do
		return
	}

	var (
		now          = e.timeService.GetTimeNow().Unix()
		height, _    = vgcontext.BlockHeightFromContext(ctx)
		txhash, _    = vgcontext.TxHashFromContext(ctx)
		id           = crypto.HashStrToHex(fmt.Sprintf("%v%v", txhash, height))
		stakeLinking *types.StakeLinking
	)

	// manually send funds from the general account to the locked for staking
	if transfer.FromAccountType == types.AccountTypeGeneral && transfer.ToAccountType == types.AccountTypeLockedForStaking {
		stakeLinking = &types.StakeLinking{
			ID:              id,
			Type:            types.StakeLinkingTypeDeposited,
			TS:              now,
			Party:           transfer.From,
			Amount:          transfer.Amount.Clone(),
			Status:          types.StakeLinkingStatusAccepted,
			FinalizedAt:     now,
			TxHash:          txhash,
			BlockHeight:     height,
			BlockTime:       now,
			LogIndex:        1,
			EthereumAddress: "",
		}
	}

	// from staking account or vested rewards, we send a remove event
	if (transfer.FromAccountType == types.AccountTypeLockedForStaking && transfer.ToAccountType == types.AccountTypeGeneral) ||
		(transfer.FromAccountType == types.AccountTypeVestedRewards && transfer.ToAccountType == types.AccountTypeGeneral) {
		stakeLinking = &types.StakeLinking{
			ID:              id,
			Type:            types.StakeLinkingTypeRemoved,
			TS:              now,
			Party:           transfer.From,
			Amount:          transfer.Amount.Clone(),
			Status:          types.StakeLinkingStatusAccepted,
			FinalizedAt:     now,
			TxHash:          txhash,
			BlockHeight:     height,
			BlockTime:       now,
			LogIndex:        1,
			EthereumAddress: "",
		}
	}

	if stakeLinking != nil {
		e.stakeAccounting.AddEvent(ctx, stakeLinking)
		e.broker.Send(events.NewStakeLinking(ctx, *stakeLinking))
	}
}

func (e *Engine) preventNebTransfers(
	party, asset string,
	from, to types.AccountType,
) error {
	isExempted := func(from, to types.AccountType) bool {
		// only a few transfers are allowed with neb to start with:
		return from == types.AccountTypeGeneral && to == types.AccountTypeLockedForStaking ||
			from == types.AccountTypeLockedForStaking && to == types.AccountTypeGeneral ||
			from == types.AccountTypeVestedRewards && to == types.AccountTypeGeneral
	}

	// if not part  of the allowlist  && the asset is $NEB
	if _, allowed := nebTransferAllowList[party]; !allowed && asset == "d1984e3d365faa05bcafbe41f50f90e3663ee7c0da22bb1e24b164e9532691b2" && !isExempted(from, to) {
		return ErrNebTransfersTemporarilyDisabled
	}

	return nil
}

func (e *Engine) oneOffTransfer(
	ctx context.Context,
	transfer *types.OneOffTransfer,
) (err error) {
	defer func() {
		if err != nil {
			e.broker.Send(events.NewOneOffTransferFundsEventWithReason(ctx, transfer, err.Error()))
		} else {
			e.broker.Send(events.NewOneOffTransferFundsEvent(ctx, transfer))
			e.updateStakingAccounts(ctx, transfer)
		}
	}()

	// ensure asset exists
	a, err := e.assets.Get(transfer.Asset)
	if err != nil {
		transfer.Status = types.TransferStatusRejected
		e.log.Debug("cannot transfer funds, invalid asset", logging.Error(err))
		return fmt.Errorf("could not transfer funds: %w", err)
	}

	if err := transfer.IsValid(); err != nil {
		transfer.Status = types.TransferStatusRejected
		return err
	}

	if err := e.preventNebTransfers(transfer.From, transfer.Asset); err != nil {
		transfer.Status = types.TransferStatusRejected
		return err
	}

	if transfer.FromDerivedKey != nil {
		if ownsDerivedKey := e.parties.CheckDerivedKeyOwnership(types.PartyID(transfer.From), *transfer.FromDerivedKey); !ownsDerivedKey {
			transfer.Status = types.TransferStatusRejected
			return fmt.Errorf("party %s does not own derived key %s", transfer.From, *transfer.FromDerivedKey)
		}
	}

	if err := e.ensureMinimalTransferAmount(a, transfer.Amount, transfer.FromAccountType, transfer.From, transfer.FromDerivedKey); err != nil {
		transfer.Status = types.TransferStatusRejected
		return err
	}

	tresps, err := e.processTransfer(
		ctx, a, transfer.From, transfer.To, "", transfer.FromAccountType,
		transfer.ToAccountType, transfer.Amount, transfer.Reference, transfer.ID, e.currentEpoch, transfer.FromDerivedKey,
		transfer,
	)
	if err != nil {
		transfer.Status = types.TransferStatusRejected
		return err
	}

	// all was OK
	transfer.Status = types.TransferStatusDone
	e.broker.Send(events.NewLedgerMovements(ctx, tresps))

	return nil
}

type timesToTransfers struct {
	deliverOn int64
	transfer  []scheduledTransfer
}

func (e *Engine) distributeScheduledTransfers(ctx context.Context, now time.Time) error {
	ttfs := []timesToTransfers{}

	// iterate over those scheduled transfers to sort them by time
	for k, v := range e.scheduledTransfers {
		if now.UnixNano() >= k {
			ttfs = append(ttfs, timesToTransfers{k, v})
			delete(e.scheduledTransfers, k)
		}
	}

	// sort slice by time.
	// no need to sort transfers they are going out as first in first out.
	sort.SliceStable(ttfs, func(i, j int) bool {
		return ttfs[i].deliverOn < ttfs[j].deliverOn
	})

	transfers := []*types.Transfer{}
	accountTypes := []types.AccountType{}
	references := []string{}
	evts := []events.Event{}
	for _, v := range ttfs {
		for _, t := range v.transfer {
			t.oneoff.Status = types.TransferStatusDone
			evts = append(evts, events.NewOneOffTransferFundsEvent(ctx, t.oneoff))
			transfers = append(transfers, t.transfer)
			accountTypes = append(accountTypes, t.accountType)
			references = append(references, t.reference)
		}
	}

	if len(transfers) <= 0 {
		// nothing to do yeay
		return nil
	}

	// at least 1 transfer updated, set to true
	tresps, err := e.col.TransferFunds(
		ctx, transfers, accountTypes, references, nil, nil, // no fees required there, they've been paid already
	)
	if err != nil {
		return err
	}

	e.broker.Send(events.NewLedgerMovements(ctx, tresps))
	e.broker.SendBatch(evts)

	return nil
}

func (e *Engine) scheduleTransfer(
	oneoff *types.OneOffTransfer,
	t *types.Transfer,
	ty types.AccountType,
	reference string,
	deliverOn time.Time,
) {
	sts, ok := e.scheduledTransfers[deliverOn.UnixNano()]
	if !ok {
		e.scheduledTransfers[deliverOn.UnixNano()] = []scheduledTransfer{}
		sts = e.scheduledTransfers[deliverOn.UnixNano()]
	}

	sts = append(sts, scheduledTransfer{
		oneoff:      oneoff,
		transfer:    t,
		accountType: ty,
		reference:   reference,
	})
	e.scheduledTransfers[deliverOn.UnixNano()] = sts
}
