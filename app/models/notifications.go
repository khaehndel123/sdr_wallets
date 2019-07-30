package models

import (
	"net/http"
)

type NewSubscription struct {
	ClientID       string `json:"client_id"`
	ResponseWriter http.ResponseWriter
	Request        *http.Request
}

type Notification struct {
	ClientID string      `json:"client_id"`
	Message  interface{} `json:"message"`
}

type TransactionCompleted struct {
	Hash   string  `json:"hash"`
	Type   string  `json:"type"`
	From   string  `json:"from"`
	To     string  `json:"to"`
	Amount float64 `json:"amount"`
}
