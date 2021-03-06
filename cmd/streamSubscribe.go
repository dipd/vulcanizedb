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
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/vulcanize/vulcanizedb/libraries/shared/streamer"
	"github.com/vulcanize/vulcanizedb/pkg/config"
	"github.com/vulcanize/vulcanizedb/pkg/core"
	"github.com/vulcanize/vulcanizedb/pkg/eth/client"
)

// streamSubscribeCmd represents the streamSubscribe command
var streamSubscribeCmd = &cobra.Command{
	Use:   "streamSubscribe",
	Short: "This command is used to subscribe to the super node stream with the provided filters",
	Long: `This command is for demo and testing purposes and is used to subscribe to the super node with the provided subscription configuration parameters.
It does not do anything with the data streamed from the super node other than unpack it and print it out for demonstration purposes.`,
	Run: func(cmd *cobra.Command, args []string) {
		subCommand = cmd.CalledAs()
		logWithCommand = *log.WithField("SubCommand", subCommand)
		streamSubscribe()
	},
}

func init() {
	rootCmd.AddCommand(streamSubscribeCmd)
}

func streamSubscribe() {
	// Prep the subscription config/filters to be sent to the server
	configureSubscription()

	// Create a new rpc client and a subscription streamer with that client
	rpcClient := getRPCClient()
	str := streamer.NewSuperNodeStreamer(rpcClient)

	// Buffered channel for reading subscription payloads
	payloadChan := make(chan streamer.SuperNodePayload, 20000)

	// Subscribe to the super node service with the given config/filter parameters
	sub, err := str.Stream(payloadChan, subscriptionConfig)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	logWithCommand.Info("awaiting payloads")
	// Receive response payloads and print out the results
	for {
		select {
		case payload := <-payloadChan:
			if payload.ErrMsg != "" {
				logWithCommand.Error(payload.ErrMsg)
				continue
			}
			for _, headerRlp := range payload.HeadersRlp {
				var header types.Header
				err = rlp.Decode(bytes.NewBuffer(headerRlp), &header)
				if err != nil {
					logWithCommand.Error(err)
					continue
				}
				fmt.Printf("Header number %d, hash %s\n", header.Number.Int64(), header.Hash().Hex())
				fmt.Printf("header: %v\n", header)
			}
			for _, trxRlp := range payload.TransactionsRlp {
				var trx types.Transaction
				buff := bytes.NewBuffer(trxRlp)
				stream := rlp.NewStream(buff, 0)
				err := trx.DecodeRLP(stream)
				if err != nil {
					logWithCommand.Error(err)
					continue
				}
				fmt.Printf("Transaction with hash %s\n", trx.Hash().Hex())
				fmt.Printf("trx: %v\n", trx)
			}
			for _, rctRlp := range payload.ReceiptsRlp {
				var rct types.ReceiptForStorage
				buff := bytes.NewBuffer(rctRlp)
				stream := rlp.NewStream(buff, 0)
				err = rct.DecodeRLP(stream)
				if err != nil {
					logWithCommand.Error(err)
					continue
				}
				fmt.Printf("Receipt with block hash %s, trx hash %s\n", rct.BlockHash.Hex(), rct.TxHash.Hex())
				fmt.Printf("rct: %v\n", rct)
				for _, l := range rct.Logs {
					if len(l.Topics) < 1 {
						logWithCommand.Error(fmt.Sprintf("log only has %d topics", len(l.Topics)))
						continue
					}
					fmt.Printf("Log for block hash %s, trx hash %s, address %s, and with topic0 %s\n",
						l.BlockHash.Hex(), l.TxHash.Hex(), l.Address.Hex(), l.Topics[0].Hex())
					fmt.Printf("log: %v\n", l)
				}
			}
			// This assumes leafs only
			for key, stateRlp := range payload.StateNodesRlp {
				var acct state.Account
				err = rlp.Decode(bytes.NewBuffer(stateRlp), &acct)
				if err != nil {
					logWithCommand.Error(err)
					continue
				}
				fmt.Printf("Account for key %s, and root %s, with balance %d\n",
					key.Hex(), acct.Root.Hex(), acct.Balance.Int64())
				fmt.Printf("state account: %v\n", acct)
			}
			for stateKey, mappedRlp := range payload.StorageNodesRlp {
				fmt.Printf("Storage for state key %s ", stateKey.Hex())
				for storageKey, storageRlp := range mappedRlp {
					fmt.Printf("with storage key %s\n", storageKey.Hex())
					var i []interface{}
					err := rlp.DecodeBytes(storageRlp, &i)
					if err != nil {
						logWithCommand.Error(err)
						continue
					}
					// if a leaf node
					if len(i) == 2 {
						keyBytes, ok := i[0].([]byte)
						if !ok {
							continue
						}
						valueBytes, ok := i[1].([]byte)
						if !ok {
							continue
						}
						fmt.Printf("Storage leaf key: %s, and value hash: %s\n",
							common.BytesToHash(keyBytes).Hex(), common.BytesToHash(valueBytes).Hex())
					}
				}
			}
		case err = <-sub.Err():
			logWithCommand.Fatal(err)
		}
	}
}

func configureSubscription() {
	logWithCommand.Info("loading subscription config")
	subscriptionConfig = config.Subscription{
		// Below default to false, which means we do not backfill by default
		BackFill:     viper.GetBool("subscription.backfill"),
		BackFillOnly: viper.GetBool("subscription.backfillOnly"),

		// Below default to 0
		// 0 start means we start at the beginning and 0 end means we continue indefinitely
		StartingBlock: big.NewInt(viper.GetInt64("subscription.startingBlock")),
		EndingBlock:   big.NewInt(viper.GetInt64("subscription.endingBlock")),

		// Below default to false, which means we get all headers by default
		HeaderFilter: config.HeaderFilter{
			Off:    viper.GetBool("subscription.headerFilter.off"),
			Uncles: viper.GetBool("subscription.headerFilter.uncles"),
		},

		// Below defaults to false and two slices of length 0
		// Which means we get all transactions by default
		TrxFilter: config.TrxFilter{
			Off: viper.GetBool("subscription.trxFilter.off"),
			Src: viper.GetStringSlice("subscription.trxFilter.src"),
			Dst: viper.GetStringSlice("subscription.trxFilter.dst"),
		},

		// Below defaults to false and one slice of length 0
		// Which means we get all receipts by default
		ReceiptFilter: config.ReceiptFilter{
			Off:       viper.GetBool("subscription.receiptFilter.off"),
			Contracts: viper.GetStringSlice("subscription.receiptFilter.contracts"),
			Topic0s:   viper.GetStringSlice("subscription.receiptFilter.topic0s"),
		},

		// Below defaults to two false, and a slice of length 0
		// Which means we get all state leafs by default, but no intermediate nodes
		StateFilter: config.StateFilter{
			Off:               viper.GetBool("subscription.stateFilter.off"),
			IntermediateNodes: viper.GetBool("subscription.stateFilter.intermediateNodes"),
			Addresses:         viper.GetStringSlice("subscription.stateFilter.addresses"),
		},

		// Below defaults to two false, and two slices of length 0
		// Which means we get all storage leafs by default, but no intermediate nodes
		StorageFilter: config.StorageFilter{
			Off:               viper.GetBool("subscription.storageFilter.off"),
			IntermediateNodes: viper.GetBool("subscription.storageFilter.intermediateNodes"),
			Addresses:         viper.GetStringSlice("subscription.storageFilter.addresses"),
			StorageKeys:       viper.GetStringSlice("subscription.storageFilter.storageKeys"),
		},
	}
}

func getRPCClient() core.RPCClient {
	vulcPath := viper.GetString("subscription.path")
	if vulcPath == "" {
		vulcPath = "ws://127.0.0.1:8080" // default to and try the default ws url if no path is provided
	}
	rawRPCClient, err := rpc.Dial(vulcPath)
	if err != nil {
		logWithCommand.Fatal(err)
	}
	return client.NewRPCClient(rawRPCClient, vulcPath)
}
