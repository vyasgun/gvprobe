package agent

import (
	"crypto/rand"
	"fmt"
	"net"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vyasgun/gvprobe/pkg/ai"
	"github.com/vyasgun/gvprobe/pkg/trace"
)

// AgentDhcpResult is one agent DHCP run (single Discover round-trip).
type AgentDhcpResult struct {
	RunNumber  int
	MAC        string
	OfferedIP  string
	LeaseState trace.LeaseState
	Error      error
}

// NewAgentDhcpCommand builds `gvprobe agent dhcp`.
func NewAgentDhcpCommand() *cobra.Command {
	return newDhcpCmd()
}

func newDhcpCmd() *cobra.Command {
	var count int
	var varyMAC bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "dhcp",
		Short: "Run DHCP Discover tests against gvproxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if count < 1 {
				return fmt.Errorf("count must be at least 1")
			}

			baseMAC, err := randomMAC()
			if err != nil {
				return err
			}

			if varyMAC {
				fmt.Printf("Agent: Running %d DHCP Discover test(s) with MAC variation\n\n", count)
			} else {
				fmt.Printf("Agent: Running %d DHCP Discover test(s)\n\n", count)
			}
			var results []AgentDhcpResult
			for i := 1; i <= count; i++ {
				mac := baseMAC
				if varyMAC {
					mac, err = incrementMAC(baseMAC, i-1)
					if err != nil {
						return err
					}
				}

				opts := trace.DhcpTraceOpts{
					MAC:   mac,
					Count: 1,
					Quiet: !verbose,
				}
				rows, err := trace.TraceDhcp(opts)
				if err != nil {
					return err
				}
				if len(rows) != 1 {
					return fmt.Errorf("expected 1 DHCP trace row, got %d", len(rows))
				}
				row := rows[0]
				results = append(results, AgentDhcpResult{
					RunNumber:  i,
					MAC:        mac,
					OfferedIP:  row.OfferedIP,
					LeaseState: row.LeaseState,
					Error:      row.Error,
				})
			}

			if !verbose {
				for _, r := range results {
					tr := agentDhcpAsTraceRow(r)
					fmt.Printf("[%d] MAC=%s → Offered IP=%s   [%s]\n", r.RunNumber, r.MAC, tr.QuietOfferedIP(), tr.QuietLeaseTag())
				}
				fmt.Println()
				printQuietAnalysis(results)
			} else {
				printAgentSummary(results)
			}
			prompt := generateClaudePrompt(results, varyMAC)
			ai.AnalyseDhcpTrace(prompt)
			return nil
		},
	}

	cmd.Flags().IntVarP(&count, "count", "c", 5, "number of DHCP Discover tests to run")
	cmd.Flags().BoolVar(&varyMAC, "vary-mac", false, "use a different client MAC for each test")
	cmd.Flags().BoolVar(&verbose, "verbose", false, "print full per-run DHCP trace output (disables trace quiet mode)")
	return cmd
}

func agentDhcpAsTraceRow(r AgentDhcpResult) trace.DhcpTraceResult {
	return trace.DhcpTraceResult{
		MAC:        r.MAC,
		OfferedIP:  r.OfferedIP,
		LeaseState: r.LeaseState,
		Error:      r.Error,
	}
}

func printQuietAnalysis(results []AgentDhcpResult) {
	n := len(results)
	var ok []AgentDhcpResult
	for _, r := range results {
		if r.Error == nil && r.OfferedIP != "" {
			ok = append(ok, r)
		}
	}
	uniq := map[string]struct{}{}
	for _, r := range ok {
		uniq[r.OfferedIP] = struct{}{}
	}
	nReused := 0
	for _, r := range ok {
		if r.LeaseState == trace.LeaseStateReused {
			nReused++
		}
	}
	var reusePct int
	if len(ok) > 0 {
		reusePct = (100 * nReused) / len(ok)
	}

	stability := "Excellent"
	fail := n - len(ok)
	if n == 0 {
		stability = "N/A"
	} else if fail == n {
		stability = "Failed"
	} else if fail > 0 {
		if fail <= n/4 {
			stability = "Degraded"
		} else {
			stability = "Poor"
		}
	}

	anomalies := quietAnomalies(results, ok)

	fmt.Println("=== LOCAL ANALYSIS ===")
	fmt.Printf("Total runs: %d\n", n)
	fmt.Printf("Unique IPs: %d\n", len(uniq))
	if len(ok) > 0 {
		fmt.Printf("Reuse rate: %d%%\n", reusePct)
	} else {
		fmt.Println("Reuse rate: N/A")
	}
	fmt.Printf("Stability: %s\n", stability)
	if len(anomalies) == 0 {
		fmt.Println("Anomalies: None")
	} else {
		fmt.Printf("Anomalies: %s\n", strings.Join(anomalies, "; "))
	}
	fmt.Println()
}

func quietAnomalies(results []AgentDhcpResult, ok []AgentDhcpResult) []string {
	var a []string
	for _, r := range results {
		if r.Error != nil {
			if r.Error == trace.ErrDhcpTimeout {
				a = append(a, fmt.Sprintf("run %d timed out", r.RunNumber))
			} else {
				a = append(a, fmt.Sprintf("run %d error: %v", r.RunNumber, r.Error))
			}
		}
	}
	ipMACs := map[string][]string{}
	for _, r := range ok {
		ipMACs[r.OfferedIP] = append(ipMACs[r.OfferedIP], r.MAC)
	}
	for ip, macs := range ipMACs {
		seen := map[string]struct{}{}
		for _, m := range macs {
			seen[m] = struct{}{}
		}
		if len(seen) > 1 {
			a = append(a, fmt.Sprintf("IP %s offered to %d distinct MACs", ip, len(seen)))
		}
	}
	return a
}

func generateClaudePrompt(results []AgentDhcpResult, varyMAC bool) string {
	prompt := "Analyze these gvprobe agent DHCP results (compact run)."
	if varyMAC {
		prompt += "MACs were varied across runs."
	}
	prompt += fmt.Sprintf("Total runs: %d\n", len(results))
	var ok int
	for _, r := range results {
		if r.Error == nil && r.OfferedIP != "" {
			ok++
		}
	}
	prompt += fmt.Sprintf("Successful offers: %d\n", ok)
	prompt += "Per run:"
	for _, r := range results {
		tr := agentDhcpAsTraceRow(r)
		prompt += fmt.Sprintf("- Run %d: MAC=%s → Offered IP=%s   [%s]\n", r.RunNumber, r.MAC, tr.QuietOfferedIP(), tr.QuietLeaseTag())
	}
	prompt += "Context:"
	prompt += "- gvproxy DHCP; lease state from gvproxy /leases vs offered YourIP."
	prompt += "- What follow-up gvprobe commands would best stress-test DHCP next?"
	return prompt
}

func printAgentSummary(results []AgentDhcpResult) {
	n := len(results)
	var ok, nReused, nNew int
	for _, r := range results {
		if r.Error != nil {
			continue
		}
		if r.OfferedIP == "" {
			continue
		}
		ok++
		switch r.LeaseState {
		case trace.LeaseStateReused:
			nReused++
		case trace.LeaseStateNew:
			nNew++
		}
	}

	fmt.Println("=== LOCAL ANALYSIS ===")
	fmt.Printf("Total runs: %d\n", n)
	switch {
	case ok == 0:
		fmt.Println("Lease behavior: No successful DHCP offers")
	case nReused == ok && nNew == 0:
		fmt.Println("Lease behavior: Consistent reuse detected")
	case nNew == ok && nReused == 0:
		fmt.Println("Lease behavior: New lease each time")
	default:
		fmt.Printf("Lease behavior: Mixed (%d reused, %d new of %d successful)\n", nReused, nNew, ok)
	}
	fmt.Println()
}

func randomMAC() (string, error) {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[0] = (buf[0] | 0x02) & 0xfe
	return net.HardwareAddr(buf).String(), nil
}

func incrementMAC(base string, offset int) (string, error) {
	hw, err := net.ParseMAC(base)
	if err != nil {
		return "", fmt.Errorf("parse MAC: %w", err)
	}
	if len(hw) != 6 {
		return "", fmt.Errorf("expected 6-byte MAC, got %d", len(hw))
	}
	carry := offset
	for i := len(hw) - 1; i >= 0 && carry > 0; i-- {
		sum := int(hw[i]) + carry
		hw[i] = byte(sum & 0xff)
		carry = sum >> 8
	}
	if carry != 0 {
		return "", fmt.Errorf("MAC overflow when incrementing")
	}
	return hw.String(), nil
}
