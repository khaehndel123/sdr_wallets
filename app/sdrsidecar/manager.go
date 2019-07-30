package sdrsidecar

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"backend/app/config"
	"backend/app/models"
)

const (
	apiKeyHeader = "x-api-key"

	ratePath = "/rate/cryptoToXdr"
)

type Manager struct {
	Config     config.SdrBackend
	HttpClient *http.Client
}

func (m *Manager) EthToSdr(ctx context.Context) (float64, error) {
	req, err := http.NewRequest("", m.Config.BasePath+ratePath, nil)
	if err != nil {
		return 0, errors.New("failed to create a get request")
	}
	req.Header.Set(apiKeyHeader, m.Config.ApiKey)

	resp, err := m.HttpClient.Do(req)
	if err != nil {
		return 0, errors.Wrap(err, "failed to perform a get request to the SDR backend")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, errors.Errorf("response has status code with error: %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, errors.Wrap(err, "failed to read a response body from the SDR backend")
	}

	rate := new(models.SdrRateResponse)
	if err = json.Unmarshal(body, rate); err != nil {
		return 0, errors.Wrap(err, "failed to unmarshal a response from the SDR backend")
	}

	for _, c := range rate.Data {
		if c.Crypto.Symbol == "ETH" {
			return c.Price, nil
		}
	}

	// not supposed to get here
	return 0, errors.New("there is no ETH rate in the SDR response")
}
