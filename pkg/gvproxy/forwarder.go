package gvproxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vyasgun/gvprobe/pkg/constants"
)

// Forward is one active port forward reported by gvproxy.
type Forward struct {
	Local    string `json:"local"`
	Remote   string `json:"remote"`
	Protocol string `json:"protocol"`
}

// ExposeRequest is the body of POST /services/forwarder/expose.
type ExposeRequest struct {
	Protocol string `json:"protocol,omitempty"` // "tcp" (default), "udp", "unix"
	Local    string `json:"local"`              // host listen address, e.g. 127.0.0.1:8080
	Remote   string `json:"remote"`             // guest destination, e.g. 192.168.127.2:80
}

// UnexposeRequest is the body of POST /services/forwarder/unexpose.
type UnexposeRequest struct {
	Protocol string `json:"protocol,omitempty"`
	Local    string `json:"local"`
}

var forwarderClient = &http.Client{Timeout: 5 * time.Second}

// ListForwards returns all active port forwards.
func ListForwards() ([]Forward, error) {
	resp, err := forwarderClient.Get(constants.GvproxyURL + "/services/forwarder/all")
	if err != nil {
		return nil, fmt.Errorf("failed to list forwards: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read forwards: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list forwards: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var forwards []Forward
	if err := json.Unmarshal(body, &forwards); err != nil {
		return nil, fmt.Errorf("failed to unmarshal forwards: %w", err)
	}
	return forwards, nil
}

// ExposeForward creates a port forward.
func ExposeForward(req ExposeRequest) error {
	return forwarderPost("/services/forwarder/expose", req)
}

// UnexposeForward removes a port forward.
func UnexposeForward(req UnexposeRequest) error {
	return forwarderPost("/services/forwarder/unexpose", req)
}

func forwarderPost(path string, body any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	resp, err := forwarderClient.Post(constants.GvproxyURL+path, "application/json", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("POST %s failed: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("POST %s: HTTP %d: %s", path, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
