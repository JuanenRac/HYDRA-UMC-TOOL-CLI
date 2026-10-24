// =============================================================================
// HYDRA-UMC-TOOL-CLI - Looking at many servers: cmd/hydra-cli/fleet.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `scan` finds HYDRA-UMC-SERVERs on a network by asking each address for the public identity page
// (GET /api/hydra-info, no credential sent), and `fleet doctor` runs the read-only `doctor` against
// every server of a list at once and gives one answer. Both only read.
//
// A scan touches many machines, so it is bounded: a range larger than --max-hosts (default 1024) is
// refused, and an address that is not on a private network (RFC 1918, loopback, link-local, IPv6
// unique-local) is refused unless --allow-public-range is given.
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"
	"time"
)

const (
	defaultScanPort        = 3000
	defaultMaxHosts        = 1024
	hardMaxHosts           = 65536
	defaultScanConcurrency = 64
	defaultProbeTimeoutMs  = 800
)

// foundServer is one HYDRA-UMC-SERVER a scan found.
type foundServer struct {
	Address          string `json:"address"`
	Port             int    `json:"port"`
	Scheme           string `json:"scheme"`
	Product          string `json:"product,omitempty"`
	Hostname         string `json:"hostname,omitempty"`
	AppVersion       string `json:"appVersion"`
	RemoteAPIVersion int    `json:"remoteApiVersion,omitempty"`
}

// parseScanTargets turns "192.168.0.0/24", "10.0.0.5" and so on into the list of addresses to probe.
func parseScanTargets(targets []string, maxHosts int, allowPublic bool) ([]netip.Addr, error) {
	var addresses []netip.Addr
	for _, target := range targets {
		var prefix netip.Prefix
		if strings.Contains(target, "/") {
			parsed, err := netip.ParsePrefix(target)
			if err != nil {
				return nil, newCliError(ExitUsageError, fmt.Errorf("%q is not a network like 192.168.0.0/24", target))
			}
			prefix = parsed.Masked()
		} else {
			addr, err := netip.ParseAddr(target)
			if err != nil {
				return nil, newCliError(ExitUsageError, fmt.Errorf("%q is neither an address nor a network like 192.168.0.0/24", target))
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		first := prefix.Addr()
		if !allowPublic && !(first.IsPrivate() || first.IsLoopback() || first.IsLinkLocalUnicast()) {
			return nil, newCliError(ExitRefused, fmt.Errorf("%s is not on a private network; scanning someone else's network is refused unless --allow-public-range is given", target))
		}
		hostBits := first.BitLen() - prefix.Bits()
		if hostBits > 20 || (1<<hostBits) > maxHosts {
			return nil, newCliError(ExitRefused, fmt.Errorf("%s has more than %d addresses; scan a narrower range, or raise --max-hosts (at most %d)", target, maxHosts, hardMaxHosts))
		}
		count := 1 << hostBits
		current := first
		for i := 0; i < count; i++ {
			// the network and broadcast addresses of an ordinary IPv4 subnet hold no server
			if first.Is4() && hostBits >= 2 && (i == 0 || i == count-1) {
				current = current.Next()
				continue
			}
			addresses = append(addresses, current)
			current = current.Next()
		}
		if len(addresses) > maxHosts {
			return nil, newCliError(ExitRefused, fmt.Errorf("the targets add up to more than %d addresses; scan fewer, or raise --max-hosts (at most %d)", maxHosts, hardMaxHosts))
		}
	}
	return addresses, nil
}

// parsePorts reads "3000,3443" into a list of ports.
func parsePorts(raw string) ([]int, error) {
	var ports []int
	for _, part := range strings.Split(raw, ",") {
		port, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || port < 1 || port > 65535 {
			return nil, newCliError(ExitUsageError, fmt.Errorf("--ports needs port numbers from 1 to 65535 separated by commas, got %q", raw))
		}
		ports = append(ports, port)
	}
	if len(ports) == 0 || len(ports) > 16 {
		return nil, newCliError(ExitUsageError, errors.New("--ports needs between 1 and 16 ports"))
	}
	return ports, nil
}

// boundedInt reads a whole-number option within a range.
func boundedInt(name, raw string, low, high, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < low || n > high {
		return 0, newCliError(ExitUsageError, fmt.Errorf("%s must be a whole number from %d to %d, got %q", name, low, high, raw))
	}
	return n, nil
}

// probeServer asks one address for its identity page. It answers false for anything that is not a HYDRA-UMC-SERVER.
func probeServer(client *http.Client, scheme string, addr netip.Addr, port int) (foundServer, bool) {
	host := net.JoinHostPort(addr.String(), strconv.Itoa(port))
	resp, err := client.Get(scheme + "://" + host + "/api/hydra-info")
	if err != nil {
		return foundServer{}, false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return foundServer{}, false
	}
	body, err := readBody(resp.Body)
	if err != nil {
		return foundServer{}, false
	}
	var info hydraInfo
	if json.Unmarshal(body, &info) != nil || strings.TrimSpace(info.AppVersion) == "" {
		return foundServer{}, false
	}
	return foundServer{
		Address: addr.String(), Port: port, Scheme: scheme,
		Product: printable(info.Product), Hostname: printable(info.Hostname),
		AppVersion: printable(info.AppVersion), RemoteAPIVersion: info.RemoteAPIVersion,
	}, true
}

// cmdScan implements `hydra-cli scan <network|address>... [options]`.
func cmdScan(w io.Writer, args []string) error {
	_, _, rest, err := resolveTarget(args)
	if err != nil {
		return err
	}
	portsRaw, rest, err := takeFlag(rest, "--ports")
	if err != nil {
		return err
	}
	scheme, rest, err := takeFlag(rest, "--scheme")
	if err != nil {
		return err
	}
	concurrencyRaw, rest, err := takeFlag(rest, "--concurrency")
	if err != nil {
		return err
	}
	maxHostsRaw, rest, err := takeFlag(rest, "--max-hosts")
	if err != nil {
		return err
	}
	probeRaw, rest, err := takeFlag(rest, "--probe-timeout-ms")
	if err != nil {
		return err
	}
	allowPublic, rest := takeSwitch(rest, "--allow-public-range")
	jsonOutput, rest := takeSwitch(rest, "--json")
	if len(rest) == 0 {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli scan <network|address>... [--ports 3000,3443] [--scheme http|https] [--json]"))
	}
	for _, target := range rest {
		if strings.HasPrefix(target, "--") {
			return newCliError(ExitUsageError, fmt.Errorf("scan does not know the option %s", target))
		}
	}
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return newCliError(ExitUsageError, fmt.Errorf("--scheme must be http or https, got %q", scheme))
	}
	if portsRaw == "" {
		portsRaw = strconv.Itoa(defaultScanPort)
	}
	ports, err := parsePorts(portsRaw)
	if err != nil {
		return err
	}
	concurrency, err := boundedInt("--concurrency", concurrencyRaw, 1, 256, defaultScanConcurrency)
	if err != nil {
		return err
	}
	maxHosts, err := boundedInt("--max-hosts", maxHostsRaw, 1, hardMaxHosts, defaultMaxHosts)
	if err != nil {
		return err
	}
	probeMs, err := boundedInt("--probe-timeout-ms", probeRaw, 50, 10000, defaultProbeTimeoutMs)
	if err != nil {
		return err
	}
	addresses, err := parseScanTargets(rest, maxHosts, allowPublic)
	if err != nil {
		return err
	}

	client := newClient(time.Duration(probeMs) * time.Millisecond)
	// a scan never follows a redirect: an address cannot send it somewhere else
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	type job struct {
		addr netip.Addr
		port int
	}
	jobs := make(chan job)
	var (
		mu    sync.Mutex
		found []foundServer
		wg    sync.WaitGroup
	)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if server, ok := probeServer(&client, scheme, j.addr, j.port); ok {
					mu.Lock()
					found = append(found, server)
					mu.Unlock()
				}
			}
		}()
	}
	for _, addr := range addresses {
		for _, port := range ports {
			jobs <- job{addr: addr, port: port}
		}
	}
	close(jobs)
	wg.Wait()

	sort.Slice(found, func(a, b int) bool {
		x, y := netip.MustParseAddr(found[a].Address), netip.MustParseAddr(found[b].Address)
		if c := x.Compare(y); c != 0 {
			return c < 0
		}
		return found[a].Port < found[b].Port
	})
	if jsonOutput {
		if found == nil {
			found = []foundServer{}
		}
		encoded, err := json.MarshalIndent(found, "", "  ")
		if err != nil {
			return newCliError(ExitGeneralError, err)
		}
		_, err = fmt.Fprintln(w, string(encoded))
		return err
	}
	if len(found) == 0 {
		fmt.Fprintf(w, "no HYDRA-UMC-SERVER found (%d addresses, %d port(s) probed)\n", len(addresses), len(ports))
		return nil
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "ADDRESS\tPORT\tVERSION\tHOSTNAME\tPRODUCT")
	for _, server := range found {
		fmt.Fprintf(table, "%s\t%d\t%s\t%s\t%s\n", server.Address, server.Port, server.AppVersion, dash(server.Hostname), dash(server.Product))
	}
	return table.Flush()
}

// ---- fleet doctor -----------------------------------------------------------

// readServerList reads a file with one server URL per line; blank lines and lines starting with # are skipped.
func readServerList(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, newCliError(ExitConfigError, fmt.Errorf("reading the server list %s: %w", path, err))
	}
	defer file.Close()
	var servers []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(io.LimitReader(file, 1<<20))
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		server, err := normalizeServer(text)
		if err != nil {
			return nil, newCliError(ExitConfigError, fmt.Errorf("%s line %d: %w", path, line, err))
		}
		if !seen[server] {
			seen[server] = true
			servers = append(servers, server)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, newCliError(ExitConfigError, fmt.Errorf("reading the server list %s: %w", path, err))
	}
	if len(servers) == 0 {
		return nil, newCliError(ExitConfigError, fmt.Errorf("the server list %s has no server", path))
	}
	if len(servers) > 256 {
		return nil, newCliError(ExitConfigError, fmt.Errorf("the server list %s has %d servers; at most 256", path, len(servers)))
	}
	return servers, nil
}

// fleetResult is the answer for one server of `fleet doctor`.
type fleetResult struct {
	Server   string        `json:"server"`
	OK       bool          `json:"ok"`
	ExitCode int           `json:"exitCode"`
	Error    string        `json:"error,omitempty"`
	Report   *doctorReport `json:"report,omitempty"`
}

// cmdFleet implements `hydra-cli fleet doctor --servers FILE [--concurrency N] [--json]`.
func cmdFleet(w io.Writer, args []string) error {
	if len(args) == 0 || args[0] != "doctor" {
		return newCliError(ExitUsageError, errors.New("usage: hydra-cli fleet doctor --servers FILE [--concurrency N] [--json]"))
	}
	_, timeout, rest, err := resolveTarget(args[1:])
	if err != nil {
		return err
	}
	listPath, rest, err := takeFlag(rest, "--servers")
	if err != nil {
		return err
	}
	concurrencyRaw, rest, err := takeFlag(rest, "--concurrency")
	if err != nil {
		return err
	}
	jsonOutput, rest := takeSwitch(rest, "--json")
	if err := rejectExtraArgs("fleet doctor", rest); err != nil {
		return err
	}
	if listPath == "" {
		return newCliError(ExitUsageError, errors.New("fleet doctor needs --servers FILE (one server URL per line)"))
	}
	concurrency, err := boundedInt("--concurrency", concurrencyRaw, 1, 64, 8)
	if err != nil {
		return err
	}
	servers, err := readServerList(listPath)
	if err != nil {
		return err
	}

	results := make([]fleetResult, len(servers))
	var wg sync.WaitGroup
	slots := make(chan struct{}, concurrency)
	for i, server := range servers {
		wg.Add(1)
		slots <- struct{}{}
		go func(i int, server string) {
			defer wg.Done()
			defer func() { <-slots }()
			client := newClient(timeout)
			report, _, _, runErr := diagnose(&client, server)
			result := fleetResult{Server: server, OK: runErr == nil, Report: &report}
			if runErr != nil {
				result.ExitCode = int(exitCodeFor(runErr))
				result.Error = printable(runErr.Error())
			}
			results[i] = result
		}(i, server)
	}
	wg.Wait()

	failed := 0
	codes := map[int]bool{}
	for _, result := range results {
		if !result.OK {
			failed++
			codes[result.ExitCode] = true
		}
	}
	if jsonOutput {
		encoded, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return newCliError(ExitGeneralError, err)
		}
		fmt.Fprintln(w, string(encoded))
	} else {
		table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
		fmt.Fprintln(table, "SERVER\tSTATUS\tVERSION\tROBOTS\tDETAIL")
		for _, result := range results {
			status, version, robots, detail := "ok", "", "", ""
			if result.Report != nil {
				version = result.Report.AppVersion
				if result.OK {
					robots = strconv.Itoa(result.Report.Robots)
				}
			}
			if !result.OK {
				status, detail = "FAIL", result.Error
			}
			fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\n", result.Server, status, dash(version), dash(robots), detail)
		}
		if err := table.Flush(); err != nil {
			return err
		}
		if failed == 0 {
			fmt.Fprintf(w, "FLEET=PASS servers=%d\n", len(results))
		} else {
			fmt.Fprintf(w, "FLEET=FAIL servers=%d ok=%d failed=%d\n", len(results), len(results)-failed, failed)
		}
	}
	if failed == 0 {
		return nil
	}
	// when every failure has the same kind its exit code is the answer; otherwise it is the general one
	code := ExitGeneralError
	if len(codes) == 1 {
		for only := range codes {
			code = ExitCode(only)
		}
	}
	return newCliError(code, fmt.Errorf("%d of %d servers failed the diagnosis", failed, len(results)))
}
