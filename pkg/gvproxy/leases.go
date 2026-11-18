package gvproxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/vyasgun/gvprobe/pkg/constants"
)

// LeaseForMAC looks up gvproxy's DHCP lease table and reports whether mac
// currently holds a lease, and which IP.
func LeaseForMAC(mac string) (found bool, leaseIP string, err error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(constants.LeasesURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to get leases: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, "", fmt.Errorf("failed to read leases: %w", err)
	}
	var leases map[string]string
	if err := json.Unmarshal(body, &leases); err != nil {
		return false, "", fmt.Errorf("failed to unmarshal leases: %w", err)
	}
	for ip, m := range leases {
		if strings.EqualFold(m, mac) {
			return true, ip, nil
		}
	}
	return false, "", nil
}
