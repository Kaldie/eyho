package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Base URLs of the KVK Handelsregister Zoeken API v2 (research.md). kvk-test is KVK's public test
// environment with fictitious companies.
const (
	KVKProductionBaseURL = "https://api.kvk.nl/api/v2"
	KVKTestBaseURL       = "https://api.kvk.nl/test/api/v2"
)

// KVK implements BusinessRegister with the Zoeken API v2. Zoeken omits inactive registrations
// unless inclusiefInactieveRegistraties=true, so the check is two-step: a hit without the flag is
// active; otherwise a hit with the flag is inactive; no hit is not found.
type KVK struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

// NewKVK creates a client. client may be nil (a 10 s timeout client is used).
func NewKVK(baseURL, apiKey string, client *http.Client) *KVK {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &KVK{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: client}
}

type zoekenResponse struct {
	Totaal     int `json:"totaal"`
	Resultaten []struct {
		KvkNummer string `json:"kvkNummer"`
		Naam      string `json:"naam"`
	} `json:"resultaten"`
	Fout []struct {
		Code         string `json:"code"`
		Omschrijving string `json:"omschrijving"`
	} `json:"fout"`
}

// Check implements BusinessRegister.
func (k *KVK) Check(ctx context.Context, kvkNumber string) (Result, error) {
	name, hit, err := k.search(ctx, kvkNumber, false)
	if err != nil {
		return Result{}, err
	}
	if hit {
		return Result{StatusActive, name}, nil
	}
	name, hit, err = k.search(ctx, kvkNumber, true)
	if err != nil {
		return Result{}, err
	}
	if hit {
		return Result{StatusInactive, name}, nil
	}
	return Result{Status: StatusNotFound}, nil
}

func (k *KVK) search(ctx context.Context, kvkNumber string, inactive bool) (string, bool, error) {
	q := url.Values{}
	q.Set("kvkNummer", kvkNumber)
	if inactive {
		q.Set("inclusiefInactieveRegistraties", "true")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.baseURL+"/zoeken?"+q.Encode(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("apikey", k.apiKey)
	req.Header.Set("Accept", "application/json")
	resp, err := k.client.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("kvk request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", false, fmt.Errorf("kvk read: %w", err)
	}
	var out zoekenResponse
	parsed := json.Unmarshal(body, &out) == nil

	switch {
	case resp.StatusCode == http.StatusOK:
		if !parsed {
			return "", false, fmt.Errorf("kvk: unparseable 200 response")
		}
		for _, r := range out.Resultaten {
			if r.KvkNummer == kvkNumber {
				return r.Naam, true, nil
			}
		}
		return "", false, nil
	case resp.StatusCode == http.StatusNotFound && parsed && len(out.Fout) > 0:
		// KVK reports "no results" as a 404 with an error list. A bare 404 (wrong path) is not
		// a definite answer and falls through to the error below.
		return "", false, nil
	}
	// 401/403 (bad key), 429, 5xx, bare 404 ...: no answer.
	return "", false, fmt.Errorf("kvk: unexpected status %d", resp.StatusCode)
}
