package eth

import (
	"math/big"
	"regexp"

	"github.com/ethereum/go-ethereum/common"
	"github.com/shopspring/decimal"
)

var (
	addressRegex = regexp.MustCompile("^0x[0-9a-fA-F]{40}$")
)

func CalcGasCost(gasLimit uint64, gasPrice *big.Int) *big.Int {
	gasLimitBig := big.NewInt(int64(gasLimit))
	return gasLimitBig.Mul(gasLimitBig, gasPrice)
}

func IsValidAddress(iaddress interface{}) bool {
	switch v := iaddress.(type) {
	case string:
		return addressRegex.MatchString(v)
	case common.Address:
		return addressRegex.MatchString(v.Hex())
	default:
		return false
	}
}

func ToETH(ivalue interface{}, decimals uint8) decimal.Decimal {
	value := new(big.Int)
	switch v := ivalue.(type) {
	case string:
		value.SetString(v, 10)
	case *big.Int:
		value = v
	}

	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
	num, _ := decimal.NewFromString(value.String())
	result := num.Div(mul)

	return result
}

func ToWei(iamount interface{}, decimals uint8) *big.Int {
	amount := decimal.NewFromFloat(0)
	switch v := iamount.(type) {
	case string:
		amount, _ = decimal.NewFromString(v)
	case float64:
		amount = decimal.NewFromFloat(v)
	case int64:
		amount = decimal.NewFromFloat(float64(v))
	case decimal.Decimal:
		amount = v
	case *decimal.Decimal:
		amount = *v
	}

	mul := decimal.NewFromFloat(float64(10)).Pow(decimal.NewFromFloat(float64(decimals)))
	result := amount.Mul(mul)

	wei := new(big.Int)
	wei.SetString(result.String(), 10)

	return wei
}
