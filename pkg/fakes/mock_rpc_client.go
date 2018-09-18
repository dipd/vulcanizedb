package fakes

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/p2p"
	. "github.com/onsi/gomega"

	"github.com/vulcanize/vulcanizedb/pkg/core"
)

type MockRpcClient struct {
	callContextErr   error
	ipcPath          string
	nodeType         core.NodeType
	passedContext    context.Context
	passedMethod     string
	passedResult     interface{}
	supportedModules map[string]string
}

func NewMockRpcClient() *MockRpcClient {
	return &MockRpcClient{}
}

func (client *MockRpcClient) SetIpcPath(ipcPath string) {
	client.ipcPath = ipcPath
}

func (client *MockRpcClient) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
	client.passedContext = ctx
	client.passedResult = result
	client.passedMethod = method
	switch method {
	case "admin_nodeInfo":
		if p, ok := result.(*p2p.NodeInfo); ok {
			p.ID = "enode://GethNode@172.17.0.1:30303"
			p.Name = "Geth/v1.7"
		}
	case "eth_getBlockByNumber":
		if p, ok := result.(*types.Header); ok {
			*p = types.Header{Number: big.NewInt(123)}
		}
		if p, ok := result.(*core.POAHeader); ok {
			n := hexutil.Big(*big.NewInt(123))
			*p = core.POAHeader{Number: &n}
		}
		if client.callContextErr != nil {
			return client.callContextErr
		}
	case "parity_versionInfo":
		if p, ok := result.(*core.ParityNodeInfo); ok {
			*p = core.ParityNodeInfo{
				Track: "",
				ParityVersion: core.ParityVersion{
					Major: 1,
					Minor: 2,
					Patch: 3,
				},
				Hash: "",
			}
		}
	case "parity_enode":
		if p, ok := result.(*string); ok {
			*p = "enode://ParityNode@172.17.0.1:30303"
		}
	case "net_version":
		if p, ok := result.(*string); ok {
			*p = "1234"
		}
	}
	return nil
}

func (client *MockRpcClient) IpcPath() string {
	return client.ipcPath
}

func (client *MockRpcClient) SupportedModules() (map[string]string, error) {
	return client.supportedModules, nil
}

func (client *MockRpcClient) SetSupporedModules(supportedModules map[string]string) {
	client.supportedModules = supportedModules
}

func (client *MockRpcClient) SetCallContextErr(err error) {
	client.callContextErr = err
}

func (client *MockRpcClient) AssertCallContextCalledWith(ctx context.Context, result interface{}, method string) {
	Expect(client.passedContext).To(Equal(ctx))
	Expect(client.passedResult).To(BeAssignableToTypeOf(result))
	Expect(client.passedMethod).To(Equal(method))
}