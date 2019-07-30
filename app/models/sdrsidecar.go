package models

type SdrRateResponseItem struct {
	Price  float64 `json:"price"`
	Crypto struct {
		Symbol string `json:"symbol"`
	} `json:"crypto"`
}

type SdrRateResponse struct {
	Data []*SdrRateResponseItem `json:"data"`
}
