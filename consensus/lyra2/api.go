package lyra2

import (
	"errors"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

var errLyra2Stopped = errors.New("lyra2 stopped")

// API exposes lyra2 related methods for the RPC interface.
type API struct {
	lyra2 *Lyra2
}

// GetWork returns a work package for external miner.
//
// The work package consists of 3 strings:
//
//	result[0], 32 bytes hex encoded current block header pow-hash
//	result[1], hex encoded header
//	result[2], 32 bytes hex encoded boundary condition ("target"), 2^256/difficulty
//	result[3], hex encoded block number
//   result[4], 32 bytes hex encoded parent block header pow-hash
//   result[5], hex encoded gas limit
//   result[6], hex encoded gas used
//   result[7], hex encoded transaction count
//   result[8], hex encoded uncle count
//   result[9], RLP encoded header with additonal empty extra data bytes
func (api *API) GetWork() ([10]string, error) {
	if api.lyra2.remote == nil {
		return [10]string{}, errors.New("not supported")
	}

	var (
		workCh = make(chan [10]string, 1)
		errc   = make(chan error, 1)
	)
	select {
	case api.lyra2.remote.fetchWorkCh <- &sealWork{errc: errc, res: workCh}:
	case <-api.lyra2.remote.exitCh:
		return [10]string{}, errLyra2Stopped
	}
	select {
	case work := <-workCh:
		return work, nil
	case err := <-errc:
		return [10]string{}, err
	}
}

// SubmitWork can be used by external miner to submit their POW solution.
// It returns an indication if the work was accepted.
// Note either an invalid solution, a stale work a non-existent work will return false.
func (api *API) SubmitWork(nonce types.BlockNonce, hash, digest common.Hash, extraNonceStr *string) bool {
	if api.lyra2.remote == nil {
		return false
	}

	var extraNonce []byte
	if extraNonceStr != nil {
		var err error
		extraNonce, err = hexutil.Decode(*extraNonceStr)
		if err != nil {
			return false
		}
	}

	var errc = make(chan error, 1)
	var blockHashCh = make(chan common.Hash, 1)
	select {
	case api.lyra2.remote.submitWorkCh <- &mineResult{
		nonce:       nonce,
		mixDigest:   digest,
		hash:        hash,
		extraNonce:  extraNonce,
		errc:        errc,
		blockHashCh: blockHashCh,
	}:
	case <-api.lyra2.remote.exitCh:
		return false
	}
	select {
	case <-errc:
		return false
	case <-blockHashCh:
		return true
	}
}

// SubmitWorkDetail is similar to eth_submitWork but will return the block hash on success,
// and return an explicit error message on failure.
//
// Params (same as `eth_submitWork`):
//   [
//       "<nonce>",
//       "<pow_hash>",
//       "<mix_hash>"
//   ]
//
// Result on success:
//   "block_hash"
//
// Error on failure:
//   {code: -32005, message: "Cannot submit work.", data: "<reason for submission failure>"}
//
// See the original proposal here: <https://github.com/paritytech/parity-ethereum/pull/9404>
//
func (api *API) SubmitWorkDetail(nonce types.BlockNonce, hash, digest common.Hash, extraNonceStr *string) (blockHash common.Hash, err rpc.ErrorWithInfo) {
	if api.lyra2.remote == nil {
		err = cannotSubmitWorkError{"not supported"}
		return
	}

	var extraNonce []byte
	if extraNonceStr != nil {
		var errorDecode error
		extraNonce, errorDecode = hexutil.Decode(*extraNonceStr)
		if errorDecode != nil {
			err = cannotSubmitWorkError{"invalid extra nonce"}
			return
		}
	}

	var errc = make(chan error, 1)
	var blockHashCh = make(chan common.Hash, 1)

	select {
	case api.lyra2.remote.submitWorkCh <- &mineResult{
		nonce:       nonce,
		mixDigest:   digest,
		hash:        hash,
		errc:        errc,
		extraNonce:  extraNonce,
		blockHashCh: blockHashCh,
	}:
	case <-api.lyra2.remote.exitCh:
		err = cannotSubmitWorkError{errLyra2Stopped.Error()}
		return
	}

	select {
	case submitErr := <-errc:
		err = cannotSubmitWorkError{submitErr.Error()}
		return
	case blockHash = <-blockHashCh:
		return
	}
}

// SubmitHashrate can be used for remote miners to submit their hash rate.
// This enables the node to report the combined hash rate of all miners
// which submit work through this node.
//
// It accepts the miner hash rate and an identifier which must be unique
// between nodes.
func (api *API) SubmitHashRate(rate hexutil.Uint64, id common.Hash) bool {
	if api.lyra2.remote == nil {
		return false
	}

	var done = make(chan struct{}, 1)
	select {
	case api.lyra2.remote.submitRateCh <- &hashrate{done: done, rate: uint64(rate), id: id}:
	case <-api.lyra2.remote.exitCh:
		return false
	}

	// Block until hash rate submitted successfully.
	<-done
	return true
}

// GetHashrate returns the current hashrate for local CPU miner and remote miner.
func (api *API) GetHashrate() uint64 {
	return uint64(api.lyra2.Hashrate())
}
