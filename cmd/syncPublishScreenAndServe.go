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
	"os"
	"path/filepath"
	syn "sync"

	"github.com/vulcanize/vulcanizedb/pkg/seed_node"

	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/vulcanizedb/pkg/ipfs"
	"github.com/vulcanize/vulcanizedb/utils"
)

// syncPublishScreenAndServeCmd represents the syncPublishScreenAndServe command
var syncPublishScreenAndServeCmd = &cobra.Command{
	Use:   "syncPublishScreenAndServe",
	Short: "Syncs all Ethereum data into IPFS, indexing the CIDs, and uses this to serve data requests to requesting clients",
	Long: `This command works alongside a modified geth node which streams
all block and state (diff) data over a websocket subscription. This process 
then converts the eth data to IPLD objects and publishes them to IPFS. Additionally,
it maintains a local index of the IPLD objects' CIDs in Postgres. It then opens up a server which 
relays relevant data to requesting clients.`,
	Run: func(cmd *cobra.Command, args []string) {
		syncPublishScreenAndServe()
	},
}

func init() {
	rootCmd.AddCommand(syncPublishScreenAndServeCmd)
}

func syncPublishScreenAndServe() {
	blockChain, ethClient, rpcClient := getBlockChainAndClients()

	db := utils.LoadPostgres(databaseConfig, blockChain.Node())
	quitChan := make(chan bool, 1)

	ipfsPath := viper.GetString("client.ipfsPath")
	if ipfsPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		ipfsPath = filepath.Join(home, ".ipfs")
	}
	processor, err := seed_node.NewProcessor(ipfsPath, &db, ethClient, rpcClient, quitChan)
	if err != nil {
		log.Fatal(err)
	}

	wg := &syn.WaitGroup{}
	forwardPayloadChan := make(chan ipfs.IPLDPayload, 20000)
	forwardQuitChan := make(chan bool, 1)
	err = processor.SyncAndPublish(wg, forwardPayloadChan, forwardQuitChan)
	if err != nil {
		log.Fatal(err)
	}
	processor.ScreenAndServe(forwardPayloadChan, forwardQuitChan)

	var ipcPath string
	ipcPath = viper.GetString("server.ipcPath")
	if ipcPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal(err)
		}
		ipcPath = filepath.Join(home, ".vulcanize/vulcanize.ipc")
	}
	_, _, err = rpc.StartIPCEndpoint(ipcPath, processor.APIs())
	if err != nil {
		log.Fatal(err)
	}

	var wsEndpoint string
	wsEndpoint = viper.GetString("server.wsEndpoint")
	if wsEndpoint == "" {
		wsEndpoint = "127.0.0.1:80"
	}
	var exposeAll = true
	var wsOrigins []string = nil
	_, _, err = rpc.StartWSEndpoint(wsEndpoint, processor.APIs(), []string{"vulcanizedb"}, wsOrigins, exposeAll)
	if err != nil {
		log.Fatal(err)
	}
	wg.Wait()
}