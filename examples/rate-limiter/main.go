// Copyright 2021-2025 The contributors of Garcon.
// This file is part of Garcon, web+API server toolkit under the MIT License.
// SPDX-License-Identifier: MIT

// this example is an adaptation of this usage:
// https://github.com/teal-finance/rainbow/blob/main/pkg/provider/deribit/deribit.go
package main

import (
	"fmt"
	"time"

	"github.com/LynxAIeu/garcon"

	"github.com/LynxAIeu/emo"
)

type (
	instrumentsResult struct {
		Result []instrument `json:"result"`
	}

	instrument struct {
		OptionType           string  `json:"option_type"`
		InstrumentName       string  `json:"instrument_name"`
		Kind                 string  `json:"kind"`
		SettlementPeriod     string  `json:"settlement_period"`
		QuoteCurrency        string  `json:"quote_currency"`
		BaseCurrency         string  `json:"base_currency"`
		MinTradeAmount       float64 `json:"min_trade_amount"`
		MakerCommission      float64 `json:"maker_commission"`
		Strike               float64 `json:"strike"`
		TickSize             float64 `json:"tick_size"`
		TakerCommission      float64 `json:"taker_commission"`
		ExpirationTimestamp  int64   `json:"expiration_timestamp"`
		CreationTimestamp    int64   `json:"creation_timestamp"`
		ContractSize         float64 `json:"contract_size"`
		BlockTradeCommission float64 `json:"block_trade_commission"`
		IsActive             bool    `json:"is_active"`
	}
)

const (
	// AdaptiveMinSleepTime to rate limit the Deribit API.
	// https://www.deribit.com/kb/deribit-rate-limits
	// "Each sub-account has a rate limit of 100 in a burst or 20 requests per second".
	adaptiveMinSleepTime = 1 * time.Millisecond

	// Hour at which the options expires = 8:00 UTC.
	Hour = 8

	// MaxBytesToRead prevents wasting memory/CPU when receiving an abnormally huge response from Deribit API.
	maxBytesToRead = 2_000_000
)

var log = emo.NewZone("app")

func main() {
	ar := garcon.NewAdaptiveRate("Deribit", adaptiveMinSleepTime)
	count := 0
	for range 1000 {
		instruments, err := query(ar, "BTC")
		if err != nil {
			log.Fatal(err)
		}
		count += instruments
		instruments, err = query(ar, "ETH")
		if err != nil {
			log.Fatal(err)
		}
		count += instruments
		instruments, err = query(ar, "SOL")
		if err != nil {
			log.Fatal(err)
		}
		count += instruments
	}
	fmt.Printf("fetched %d instruments from Deribit \n", count)
}

func query(ar garcon.AdaptiveRate, coin string) (int, error) {
	const api = "https://deribit.com/api/v2/public/get_instruments?currency="
	const opts = "&expired=false&kind=option"
	url := api + coin + opts
	log.Info("Deribit " + url)

	var result instrumentsResult
	err := ar.Get(coin, url, &result, maxBytesToRead)
	if err != nil {
		return 0, err
	}

	return len(result.Result), nil
}
