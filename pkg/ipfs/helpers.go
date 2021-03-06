// VulcanizeDB
// Copyright © 2019 Vulcanize

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package ipfs

import (
	"context"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-ipfs/core"
	"github.com/ipfs/go-ipfs/plugin/loader"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

// InitIPFSPlugins is used to initialized IPFS plugins before creating a new IPFS node
// This should only be called once
func InitIPFSPlugins() error {
	l, err := loader.NewPluginLoader("")
	if err != nil {
		return err
	}
	err = l.Initialize()
	if err != nil {
		return err
	}
	return l.Inject()
}

// InitIPFSBlockService is used to configure and return a BlockService using an ipfs repo path (e.g. ~/.ipfs)
func InitIPFSBlockService(ipfsPath string) (blockservice.BlockService, error) {
	r, openErr := fsrepo.Open(ipfsPath)
	if openErr != nil {
		return nil, openErr
	}
	ctx := context.Background()
	cfg := &core.BuildCfg{
		Online: false,
		Repo:   r,
	}
	ipfsNode, newNodeErr := core.NewNode(ctx, cfg)
	if newNodeErr != nil {
		return nil, newNodeErr
	}
	return ipfsNode.Blocks, nil
}

// AddressToKey hashes an address
func AddressToKey(address common.Address) common.Hash {
	return crypto.Keccak256Hash(address[:])
}

// HexToKey hashes a hex (0x leading or not) string
func HexToKey(hex string) common.Hash {
	addr := common.FromHex(hex)
	return crypto.Keccak256Hash(addr[:])
}

// EmptyCIDWrapper returns whether or not the provided CIDWrapper has any Cids we need to process
func EmptyCIDWrapper(cids CIDWrapper) bool {
	if len(cids.Transactions) > 0 || len(cids.Headers) > 0 || len(cids.Uncles) > 0 || len(cids.Receipts) > 0 || len(cids.StateNodes) > 0 || len(cids.StorageNodes) > 0 {
		return false
	}
	return true
}
