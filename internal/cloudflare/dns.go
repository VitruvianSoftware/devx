// Copyright (c) 2026 VitruvianSoftware
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const cfAPIBase = "https://api.cloudflare.com/client/v4"

// getAPIToken reads the Cloudflare API token from the environment.
func getAPIToken() (string, error) {
	token := os.Getenv("CLOUDFLARE_API_TOKEN")
	if token == "" {
		token = os.Getenv("CF_API_TOKEN")
	}
	if token == "" {
		return "", fmt.Errorf("CLOUDFLARE_API_TOKEN or CF_API_TOKEN environment variable is required")
	}
	return token, nil
}

// cfAPIResponse is the standard Cloudflare API v4 response wrapper.
type cfAPIResponse struct {
	Success bool            `json:"success"`
	Errors  []cfAPIError    `json:"errors"`
	Result  json.RawMessage `json:"result"`
}

type cfAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cfZone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type cfDNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

// cfRequest makes an authenticated request to the Cloudflare API.
func cfRequest(method, path string, body interface{}) (*cfAPIResponse, error) {
	token, err := getAPIToken()
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, cfAPIBase+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare API request: %w", err)
	}
	defer resp.Body.Close()

	var apiResp cfAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if !apiResp.Success {
		if len(apiResp.Errors) > 0 {
			return &apiResp, fmt.Errorf("cloudflare API error: %s (code %d)", apiResp.Errors[0].Message, apiResp.Errors[0].Code)
		}
		return &apiResp, fmt.Errorf("cloudflare API returned success=false")
	}

	return &apiResp, nil
}

// LookupZoneID finds the zone ID for a given domain.
// For "devx.vitruviansoftware.dev", this looks up "vitruviansoftware.dev".
func LookupZoneID(zoneName string) (string, error) {
	resp, err := cfRequest("GET", fmt.Sprintf("/zones?name=%s&status=active", zoneName), nil)
	if err != nil {
		return "", err
	}

	var zones []cfZone
	if err := json.Unmarshal(resp.Result, &zones); err != nil {
		return "", fmt.Errorf("parse zones: %w", err)
	}
	if len(zones) == 0 {
		return "", fmt.Errorf("zone %q not found in your Cloudflare account", zoneName)
	}
	return zones[0].ID, nil
}

// CreateCNAME creates or updates a CNAME record in the given zone.
func CreateCNAME(zoneID, recordName, target string, proxied bool) error {
	// Check for existing record first
	resp, err := cfRequest("GET", fmt.Sprintf("/zones/%s/dns_records?type=CNAME&name=%s", zoneID, recordName), nil)
	if err != nil {
		return err
	}

	var existing []cfDNSRecord
	if err := json.Unmarshal(resp.Result, &existing); err != nil {
		return fmt.Errorf("parse existing records: %w", err)
	}

	payload := map[string]interface{}{
		"type":    "CNAME",
		"name":    recordName,
		"content": target,
		"proxied": proxied,
		"ttl":     1, // Auto
	}

	if len(existing) > 0 {
		// Update existing record
		_, err = cfRequest("PUT", fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, existing[0].ID), payload)
		return err
	}

	// Create new record
	_, err = cfRequest("POST", fmt.Sprintf("/zones/%s/dns_records", zoneID), payload)
	return err
}

// CreateTXT creates or updates a TXT record in the given zone.
// Used for domain verification (e.g., GitHub Pages challenge records).
func CreateTXT(zoneID, recordName, value string) error {
	// Check for existing TXT record
	resp, err := cfRequest("GET", fmt.Sprintf("/zones/%s/dns_records?type=TXT&name=%s", zoneID, recordName), nil)
	if err != nil {
		return err
	}

	var existing []cfDNSRecord
	if err := json.Unmarshal(resp.Result, &existing); err != nil {
		return fmt.Errorf("parse existing TXT records: %w", err)
	}

	payload := map[string]interface{}{
		"type":    "TXT",
		"name":    recordName,
		"content": value,
		"ttl":     1, // Auto
	}

	if len(existing) > 0 {
		_, err = cfRequest("PUT", fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, existing[0].ID), payload)
		return err
	}

	_, err = cfRequest("POST", fmt.Sprintf("/zones/%s/dns_records", zoneID), payload)
	return err
}
