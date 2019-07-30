package sdrsidecar

import "context"

type Service interface {
	EthToSdr(ctx context.Context) (float64, error)
}
