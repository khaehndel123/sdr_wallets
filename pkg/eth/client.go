package eth

import (
	"context"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
)

func Dial(rawurl string) (*ethclient.Client, *rpc.Client, error) {
	rpcClient, err := rpc.DialContext(context.Background(), rawurl)
	if err != nil {
		return nil, nil, err
	}
	return ethclient.NewClient(rpcClient), rpcClient, nil
}
