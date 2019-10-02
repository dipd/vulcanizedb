// Copyright © 2019 Vulcanize, Inc
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"github.com/vulcanize/vulcanizedb/pkg/ipfs"
	"os"
	"path/filepath"
	syn "sync"
	"time"

	"github.com/vulcanize/vulcanizedb/pkg/super_node"

	"github.com/spf13/viper"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/geth"
	"github.com/vulcanize/vulcanizedb/pkg/geth/client"
	vRpc "github.com/vulcanize/vulcanizedb/pkg/geth/converters/rpc"
	"github.com/vulcanize/vulcanizedb/pkg/geth/node"
	"github.com/vulcanize/vulcanizedb/utils"
)

// syncAndPublishCmd represents the syncAndPublish command
var syncAndPublishCmd = &cobra.Command{
	Use:   "syncAndPublish",
	Short: "Syncs all Ethereum data into IPFS, indexing the CIDs",
	Long: `This command works alongside a modified geth node which streams
all block and state (diff) data over a websocket subscription. This process 
then converts the eth data to IPLD objects and publishes them to IPFS. Additionally,
it maintains a local index of the IPLD objects' CIDs in Postgres.`,
	Run: func(cmd *cobra.Command, args []string) {
		syncAndPublish()
	},
}

var ipfsPath string

func init() {
	rootCmd.AddCommand(syncAndPublishCmd)
}

func syncAndPublish() {
	superNode, err := newSuperNode()
	if err != nil {
		log.Fatal(err)
	}
	wg := &syn.WaitGroup{}
	err = superNode.SyncAndPublish(wg, nil, nil)
	if err != nil {
		log.Fatal(err)
	}
	if viper.GetBool("backfill.on") && viper.GetString("backfill.ipcPath") != "" {
		backfiller := newBackFiller(superNode.GetPublisher())
		if err != nil {
			log.Fatal(err)
		}
		backfiller.FillGaps(wg, nil)
	}
	wg.Wait() // If an error was thrown, wg.Add was never called and this will fall through
}

func getBlockChainAndClient(path string) (*geth.BlockChain, core.RpcClient) {
	rawRpcClient, err := rpc.Dial(path)
	if err != nil {
		log.Fatal(err)
	}
	rpcClient := client.NewRpcClient(rawRpcClient, ipc)
	ethClient := ethclient.NewClient(rawRpcClient)
	vdbEthClient := client.NewEthClient(ethClient)
	vdbNode := node.MakeNode(rpcClient)
	transactionConverter := vRpc.NewRpcTransactionConverter(ethClient)
	blockChain := geth.NewBlockChain(vdbEthClient, rpcClient, vdbNode, transactionConverter)
	return blockChain, rpcClient
}

func newSuperNode() (super_node.NodeInterface, error) {
	blockChain, rpcClient := getBlockChainAndClient(ipc)
	db := utils.LoadPostgres(databaseConfig, blockChain.Node())
	quitChan := make(chan bool)
	ipfsPath = viper.GetString("client.ipfsPath")
	if ipfsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		ipfsPath = filepath.Join(home, ".ipfs")
	}
	workers := viper.GetInt("client.workers")
	if workers < 1 {
		workers = 1
	}
	return super_node.NewSuperNode(ipfsPath, &db, rpcClient, quitChan, workers, blockChain.Node())
}

func newBackFiller(ipfsPublisher ipfs.IPLDPublisher) super_node.BackFillInterface {
	blockChain, archivalRpcClient := getBlockChainAndClient(viper.GetString("backfill.ipcPath"))
	db := utils.LoadPostgres(databaseConfig, blockChain.Node())
	freq := viper.GetInt("backfill.frequency")
	var frequency time.Duration
	if freq <= 0 {
		frequency = time.Minute * 5
	} else {
		frequency = time.Duration(freq)
	}
	return super_node.NewBackFillService(ipfsPublisher, &db, archivalRpcClient, time.Minute*frequency)
}
