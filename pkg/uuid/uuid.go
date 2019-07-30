package uuid

import (
	"github.com/segmentio/ksuid"
)

func NewUUID() string {
	return ksuid.New().String()
}
