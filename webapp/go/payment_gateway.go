package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
)

var erroredUpstream = errors.New("errored upstream")

var httpClient *http.Client

func init() {
	dt := http.DefaultTransport.(*http.Transport).Clone()
	dt.MaxIdleConns = 3000
	dt.MaxIdleConnsPerHost = 500
	httpClient = &http.Client{
		Transport: dt,
	}

}

type paymentGatewayPostPaymentRequest struct {
	Amount int `json:"amount"`
}

type paymentGatewayGetPaymentsResponseOne struct {
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

func requestPaymentGatewayPostPayment(ctx context.Context, paymentGatewayURL string, token string, param *paymentGatewayPostPaymentRequest, retrieveRidesOrderByCreatedAtAsc func() ([]Ride, error)) error {
	b, err := json.Marshal(param)
	if err != nil {
		return err
	}

	// 失敗したらとりあえずリトライ
	// FIXME: 社内決済マイクロサービスのインフラに異常が発生していて、同時にたくさんリクエストすると変なことになる可能性あり
	operation := func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentGatewayURL+"/payments", bytes.NewBuffer(b))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)

		res, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		io.Copy(io.Discard, res.Body)
		res.Body.Close()

		if res.StatusCode != http.StatusNoContent {
			// エラーが返ってきても成功している場合があるので、社内決済マイクロサービスに問い合わせ
			getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, paymentGatewayURL+"/payments", bytes.NewBuffer([]byte{}))
			if err != nil {
				return err
			}
			getReq.Header.Set("Authorization", "Bearer "+token)

			getRes, err := httpClient.Do(getReq)
			if err != nil {
				return err
			}

			// GET /payments は障害と関係なく200が返るので、200以外は回復不能なエラーとする
			if getRes.StatusCode != http.StatusOK {
				return fmt.Errorf("[GET /payments] unexpected status code (%d)", getRes.StatusCode)
			}
			var payments []paymentGatewayGetPaymentsResponseOne
			if err := json.NewDecoder(getRes.Body).Decode(&payments); err != nil {
				getRes.Body.Close()
				return err
			}
			getRes.Body.Close()

			rides, err := retrieveRidesOrderByCreatedAtAsc()
			if err != nil {
				return err
			}

			if len(rides) != len(payments) {
				return fmt.Errorf("unexpected number of payments: %d != %d. %w", len(rides), len(payments), erroredUpstream)
			}

			return nil
		}
		return nil
	}

	if err := backoff.Retry(operation, backoff.WithMaxRetries(backoff.NewExponentialBackOff(backoff.WithInitialInterval(50*time.Millisecond)), 5)); err != nil {
		return err
	}

	return nil
}
