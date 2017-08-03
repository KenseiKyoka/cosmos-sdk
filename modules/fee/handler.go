package fee

import (
	"github.com/tendermint/basecoin"
	"github.com/tendermint/basecoin/errors"
	"github.com/tendermint/basecoin/modules/coin"
	"github.com/tendermint/basecoin/stack"
	"github.com/tendermint/basecoin/state"
)

// NameFee - namespace for the fee module
const NameFee = "fee"

// Bank is a default location for the fees, but pass anything into
// the middleware constructor
var Bank = basecoin.Actor{App: NameFee, Address: []byte("bank")}

// SimpleFeeMiddleware - middleware for fee checking, constant amount
// It used modules.coin to move the money
type SimpleFeeMiddleware struct {
	// the fee must be the same denomination and >= this amount
	// if the amount is 0, then the fee tx wrapper is optional
	MinFee coin.Coin
	// all fees go here, which could be a dump (Bank) or something reachable
	// by other app logic
	Collector basecoin.Actor
	stack.PassOption
}

var _ stack.Middleware = SimpleFeeMiddleware{}

// NewSimpleFeeMiddleware returns a fee handler with a fixed minimum fee.
//
// If minFee is 0, then the FeeTx is optional
func NewSimpleFeeMiddleware(minFee coin.Coin, collector basecoin.Actor) SimpleFeeMiddleware {
	return SimpleFeeMiddleware{
		MinFee:    minFee,
		Collector: collector,
	}
}

// Name - return the namespace for the fee module
func (SimpleFeeMiddleware) Name() string {
	return NameFee
}

// CheckTx - check the transaction
func (h SimpleFeeMiddleware) CheckTx(ctx basecoin.Context, store state.SimpleDB, tx basecoin.Tx, next basecoin.Checker) (res basecoin.Result, err error) {
	return h.doTx(ctx, store, tx, next.CheckTx)
}

// DeliverTx - send the fee handler transaction
func (h SimpleFeeMiddleware) DeliverTx(ctx basecoin.Context, store state.SimpleDB, tx basecoin.Tx, next basecoin.Deliver) (res basecoin.Result, err error) {
	return h.doTx(ctx, store, tx, next.DeliverTx)
}

func (h SimpleFeeMiddleware) doTx(ctx basecoin.Context, store state.SimpleDB, tx basecoin.Tx, next basecoin.CheckerFunc) (res basecoin.Result, err error) {
	feeTx, ok := tx.Unwrap().(Fee)
	if !ok {
		// the fee wrapper is not required if there is no minimum
		if h.MinFee.IsZero() {
			return next(ctx, store, tx)
		}
		return res, errors.ErrInvalidFormat(TypeFees, tx)
	}

	// see if it is the proper denom and big enough
	fee := feeTx.Fee
	if fee.Denom != h.MinFee.Denom {
		return res, ErrWrongFeeDenom(h.MinFee.Denom)
	}
	if !fee.IsGTE(h.MinFee) {
		return res, ErrInsufficientFees()
	}

	// now, try to make a IPC call to coins...
	send := coin.NewSendOneTx(feeTx.Payer, h.Collector, coin.Coins{fee})
	_, err = next(ctx, store, send)
	if err != nil {
		return res, err
	}

	return next(ctx, store, feeTx.Tx)
}