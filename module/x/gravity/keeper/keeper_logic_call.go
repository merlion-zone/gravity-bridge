package keeper

import (
	"encoding/hex"
	"fmt"

	"github.com/cosmos/cosmos-sdk/store/prefix"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/Gravity-Bridge/Gravity-Bridge/module/x/gravity/types"
)

// ///////////////////////////
//       LOGICCALLS        //
// ///////////////////////////

// GetOutgoingLogicCall gets an outgoing logic call
func (k Keeper) GetOutgoingLogicCall(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64) *types.OutgoingLogicCall {
	store := k.SubStore(ctx)
	call := types.OutgoingLogicCall{
		Transfers:            []types.ERC20Token{},
		Fees:                 []types.ERC20Token{},
		LogicContractAddress: "",
		Payload:              []byte{},
		Timeout:              0,
		InvalidationId:       invalidationID,
		InvalidationNonce:    invalidationNonce,
		Block:                0,
	}
	k.cdc.MustUnmarshal(store.Get(types.GetOutgoingLogicCallKey(invalidationID, invalidationNonce)), &call)
	return &call
}

// SetOutogingLogicCall sets an outgoing logic call, panics if one already exists at this
// index, since we collect signatures over logic calls no mutation can be valid
func (k Keeper) SetOutgoingLogicCall(ctx sdk.Context, call types.OutgoingLogicCall) {
	store := k.SubStore(ctx)

	// Store checkpoint to prove that this logic call actually happened
	checkpoint := call.GetCheckpoint(k.GetGravityID(ctx))
	k.SetPastEthSignatureCheckpoint(ctx, checkpoint)
	key := types.GetOutgoingLogicCallKey(call.InvalidationId, call.InvalidationNonce)
	if store.Has(key) {
		panic("Can not overwrite logic call")
	}
	store.Set(key,
		k.cdc.MustMarshal(&call))
}

// DeleteOutgoingLogicCall deletes outgoing logic calls
func (k Keeper) DeleteOutgoingLogicCall(ctx sdk.Context, invalidationID []byte, invalidationNonce uint64) {
	k.SubStore(ctx).Delete(types.GetOutgoingLogicCallKey(invalidationID, invalidationNonce))
}

// IterateOutgoingLogicCalls iterates over outgoing logic calls
func (k Keeper) IterateOutgoingLogicCalls(ctx sdk.Context, cb func([]byte, types.OutgoingLogicCall) bool) {
	prefixStore := prefix.NewStore(k.SubStore(ctx), types.KeyOutgoingLogicCall)
	iter := prefixStore.Iterator(nil, nil)
	defer iter.Close()
	for ; iter.Valid(); iter.Next() {
		var call types.OutgoingLogicCall
		k.cdc.MustUnmarshal(iter.Value(), &call)
		// cb returns true to stop early
		if cb(iter.Key(), call) {
			break
		}
	}
}

// GetOutgoingLogicCalls returns the outgoing logic calls
func (k Keeper) GetOutgoingLogicCalls(ctx sdk.Context) (out []types.OutgoingLogicCall) {
	k.IterateOutgoingLogicCalls(ctx, func(_ []byte, call types.OutgoingLogicCall) bool {
		out = append(out, call)
		return false
	})
	return
}

// CancelOutgoingLogicCalls releases all TX in the batch and deletes the batch
func (k Keeper) CancelOutgoingLogicCall(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64) error {
	call := k.GetOutgoingLogicCall(ctx, invalidationId, invalidationNonce)
	if call == nil {
		return types.ErrUnknown
	}
	// Delete batch since it is finished
	k.DeleteOutgoingLogicCall(ctx, call.InvalidationId, call.InvalidationNonce)

	// a consuming application will have to watch for this event and act on it
	ctx.EventManager().EmitTypedEvent(
		&types.EventOutgoingLogicCallCanceled{
			LogicCallInvalidationId:    fmt.Sprint(call.InvalidationId),
			LogicCallInvalidationNonce: fmt.Sprint(call.InvalidationNonce),
		},
	)

	return nil
}

// ///////////////////////////
//       LOGICCONFIRMS     //
// ///////////////////////////

// SetLogicCallConfirm sets a logic confirm in the store
func (k Keeper) SetLogicCallConfirm(ctx sdk.Context, msg *types.MsgConfirmLogicCall) {
	bytes, err := hex.DecodeString(msg.InvalidationId)
	if err != nil {
		panic(err)
	}

	acc, err := sdk.AccAddressFromBech32(msg.Orchestrator)
	if err != nil {
		panic(err)
	}

	k.SubStore(ctx).
		Set(types.GetLogicConfirmKey(bytes, msg.InvalidationNonce, acc), k.cdc.MustMarshal(msg))
}

// GetLogicCallConfirm gets a logic confirm from the store
func (k Keeper) GetLogicCallConfirm(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64, val sdk.AccAddress) *types.MsgConfirmLogicCall {
	if err := sdk.VerifyAddressFormat(val); err != nil {
		ctx.Logger().Error("invalid val address")
		return nil
	}
	store := k.SubStore(ctx)
	data := store.Get(types.GetLogicConfirmKey(invalidationId, invalidationNonce, val))
	if data == nil {
		return nil
	}
	out := types.MsgConfirmLogicCall{
		InvalidationId:    "",
		InvalidationNonce: invalidationNonce,
		EthSigner:         "",
		Orchestrator:      "",
		Signature:         "",
	}
	k.cdc.MustUnmarshal(data, &out)
	return &out
}

// DeleteLogicCallConfirm deletes a logic confirm from the store
func (k Keeper) DeleteLogicCallConfirm(
	ctx sdk.Context,
	invalidationID []byte,
	invalidationNonce uint64,
	val sdk.AccAddress) {
	k.SubStore(ctx).Delete(types.GetLogicConfirmKey(invalidationID, invalidationNonce, val))
}

// IterateLogicConfirmByInvalidationIDAndNonce iterates over all logic confirms stored by nonce
func (k Keeper) IterateLogicConfirmByInvalidationIDAndNonce(
	ctx sdk.Context,
	invalidationID []byte,
	invalidationNonce uint64,
	cb func([]byte, *types.MsgConfirmLogicCall) bool) {
	store := k.SubStore(ctx)
	prefix := types.GetLogicConfirmNonceInvalidationIdPrefix(invalidationID, invalidationNonce)
	iter := store.Iterator(prefixRange([]byte(prefix)))

	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		confirm := types.MsgConfirmLogicCall{
			InvalidationId:    "",
			InvalidationNonce: invalidationNonce,
			EthSigner:         "",
			Orchestrator:      "",
			Signature:         "",
		}
		k.cdc.MustUnmarshal(iter.Value(), &confirm)
		// cb returns true to stop early
		if cb(iter.Key(), &confirm) {
			break
		}
	}
}

// GetLogicConfirmsByInvalidationIdAndNonce returns the logic call confirms
func (k Keeper) GetLogicConfirmByInvalidationIDAndNonce(ctx sdk.Context, invalidationId []byte, invalidationNonce uint64) (out []types.MsgConfirmLogicCall) {
	k.IterateLogicConfirmByInvalidationIDAndNonce(ctx, invalidationId, invalidationNonce, func(_ []byte, msg *types.MsgConfirmLogicCall) bool {
		out = append(out, *msg)
		return false
	})
	return
}
