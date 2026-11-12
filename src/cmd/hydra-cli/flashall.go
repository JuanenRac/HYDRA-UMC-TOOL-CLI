// =============================================================================
// HYDRA-UMC-TOOL-CLI - Flash every slot: cmd/hydra-cli/flashall.go
// Copyright (C) 2026 JuanenRac (Electro Hobby 3D) <electrohobby3d@gmail.com>
// GPL-3.0-or-later - see LICENSE
// =============================================================================
// `flash-all` sends one signed firmware image to the controller boards of slots 0 to 7 (or the ones named), one after the other, through
// POST /api/hardware/canota/flash of the server, which relays it to the flashing service. The CLI adds nothing to the safety of the
// flash: the signature is the one made where the private key is, the server checks its shape, and the bootloader of each board checks the
// signature and refuses a wrong target or an older image. What the CLI adds is the loop, a plan that shows what would be sent
// (--dry-run: nothing leaves the machine and no login is needed), a confirmation (--yes) and a stop at the first slot that fails.
//
// Evidence: written against the contract of the server and tested against stand-in servers (this file's tests and the fake server of
// the SDK); no board has been flashed with it.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	// maxFlashImageBytes is the largest image the server takes (its body limit is 64 MiB).
	maxFlashImageBytes = 64 << 20
	// flashSlotCount is how many slots the stack bus has.
	flashSlotCount = 8
	// defaultFlashTimeout is how long one slot may take when --timeout is not given (a flash is minutes, not seconds).
	defaultFlashTimeout = 10 * time.Minute
)

var (
	versionTriple    = regexp.MustCompile(`^(\d{1,5})\.(\d{1,5})\.(\d{1,5})$`)
	signatureHex     = regexp.MustCompile(`^[0-9a-fA-F]{128}$`)
	flashTargetName  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
	slotListItem     = regexp.MustCompile(`^(\d)(?:-(\d))?$`)
	flashAnswerPhase = regexp.MustCompile(`^[a-z_]{1,32}$`)
)

// flashPlan is everything that is going to be sent, checked before anything is.
type flashPlan struct {
	image       []byte
	imageSHA256 string
	slots       []int
	query       url.Values // everything but the slot
}

// parseSlotList reads "0-7", "0,2,5" or "1-3,6": whole numbers from 0 to 7, each once, in the order given.
func parseSlotList(text string) ([]int, error) {
	var slots []int
	seen := map[int]bool{}
	for _, part := range strings.Split(text, ",") {
		match := slotListItem.FindStringSubmatch(strings.TrimSpace(part))
		if match == nil {
			return nil, newCliError(ExitUsageError, fmt.Errorf("--slots must be numbers from 0 to %d, like 0-7 or 0,2,5, not %q", flashSlotCount-1, text))
		}
		low, _ := strconv.Atoi(match[1])
		high := low
		if match[2] != "" {
			high, _ = strconv.Atoi(match[2])
		}
		if low > high || high >= flashSlotCount {
			return nil, newCliError(ExitUsageError, fmt.Errorf("--slots %q is not a range inside 0 to %d", part, flashSlotCount-1))
		}
		for slot := low; slot <= high; slot++ {
			if seen[slot] {
				return nil, newCliError(ExitUsageError, fmt.Errorf("--slots names slot %d twice", slot))
			}
			seen[slot] = true
			slots = append(slots, slot)
		}
	}
	return slots, nil
}

// readSignature returns the signature from --signature or from the first line of --signature-file (lower case).
func readSignature(value, file string) (string, error) {
	if value != "" && file != "" {
		return "", newCliError(ExitUsageError, errors.New("give --signature or --signature-file, not both"))
	}
	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return "", newCliError(ExitConfigError, fmt.Errorf("cannot read the signature file: %w", err))
		}
		value, _, _ = strings.Cut(string(raw), "\n")
		value = strings.TrimSpace(value)
	}
	if value != "" && !signatureHex.MatchString(value) {
		return "", newCliError(ExitUsageError, errors.New("the signature must be 128 hexadecimal digits"))
	}
	return strings.ToLower(value), nil
}

// buildFlashPlan checks every option and reads the image. Nothing here touches the network.
func buildFlashPlan(p *parsedFlags) (flashPlan, error) {
	var plan flashPlan
	imagePath := p.String("--image")
	if imagePath == "" {
		return plan, newCliError(ExitUsageError, errors.New("usage: hydra-cli flash-all --image FILE --tier N --hardware-id N --image-version X.Y.Z [--slots 0-7] [--dry-run | --yes] (see hydra-cli flash-all --help)"))
	}
	info, err := os.Stat(imagePath)
	if err != nil {
		return plan, newCliError(ExitConfigError, fmt.Errorf("cannot read the image: %w", err))
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxFlashImageBytes {
		return plan, newCliError(ExitConfigError, fmt.Errorf("the image must be a regular file of 1 byte to %d MiB", maxFlashImageBytes>>20))
	}
	file, err := os.Open(imagePath)
	if err != nil {
		return plan, newCliError(ExitConfigError, fmt.Errorf("cannot read the image: %w", err))
	}
	defer file.Close()
	plan.image, err = io.ReadAll(io.LimitReader(file, maxFlashImageBytes+1))
	if err != nil || len(plan.image) == 0 || len(plan.image) > maxFlashImageBytes {
		return plan, newCliError(ExitConfigError, errors.New("the image could not be read in full"))
	}
	sum := sha256.Sum256(plan.image)
	plan.imageSHA256 = hex.EncodeToString(sum[:])

	slots := p.String("--slots")
	if slots == "" {
		slots = "0-7"
	}
	if plan.slots, err = parseSlotList(slots); err != nil {
		return plan, err
	}
	query := url.Values{}
	for _, name := range []string{"--tier", "--hardware-id"} {
		value := p.String(name)
		number, convErr := strconv.Atoi(value)
		if convErr != nil || number < 0 || number > 65535 {
			return plan, newCliError(ExitUsageError, fmt.Errorf("%s must be a whole number from 0 to 65535 (it is needed)", name))
		}
		query.Set(strings.ReplaceAll(strings.TrimPrefix(name, "--"), "-", "_"), value)
	}
	relay := p.String("--relay")
	if relay == "" {
		relay = "0"
	}
	if relay != "0" && relay != "1" {
		return plan, newCliError(ExitUsageError, errors.New("--relay must be 0 or 1"))
	}
	query.Set("relay", relay)
	version := versionTriple.FindStringSubmatch(p.String("--image-version"))
	if version == nil {
		return plan, newCliError(ExitUsageError, errors.New("--image-version must be major.minor.patch (it is needed)"))
	}
	query.Set("version_major", version[1])
	query.Set("version_minor", version[2])
	query.Set("version_patch", version[3])
	signature, err := readSignature(p.String("--signature"), p.String("--signature-file"))
	if err != nil {
		return plan, err
	}
	if signature != "" {
		query.Set("signature", signature)
	}
	for _, option := range []string{"--downgrade-from", "--min-bootloader"} {
		if value := p.String(option); value != "" {
			if !versionTriple.MatchString(value) {
				return plan, newCliError(ExitUsageError, fmt.Errorf("%s must be major.minor.patch", option))
			}
			query.Set(strings.ReplaceAll(strings.TrimPrefix(option, "--"), "-", "_"), value)
		}
	}
	if target := p.String("--target"); target != "" {
		if !flashTargetName.MatchString(target) {
			return plan, newCliError(ExitUsageError, errors.New("--target must be a short name of letters, digits, dots, dashes and underscores"))
		}
		query.Set("target", target)
	}
	plan.query = query
	return plan, nil
}

// pathFor is the request path of one slot; the order of the query is fixed so a plan is the same text every time.
func (plan flashPlan) pathFor(slot int) string {
	query := url.Values{}
	for key, values := range plan.query {
		query[key] = values
	}
	query.Set("slot", strconv.Itoa(slot))
	return "/api/hardware/canota/flash?" + query.Encode()
}

// flashSlotResult is the outcome of one slot as the command reports it.
type flashSlotResult struct {
	Slot   int    `json:"slot"`
	Status string `json:"status"` // "flashed", "failed", "refused", "not-tried" or "planned"
	Phase  string `json:"phase,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// explicitTimeout says whether --timeout is among the arguments (resolveTarget takes it out, and a flash needs a longer default).
func explicitTimeout(args []string) bool {
	for _, a := range args {
		if a == "--timeout" {
			return true
		}
	}
	return false
}

// flashOne sends the image to one slot and reads the final answer of the server.
func flashOne(client *http.Client, server, controllerID string, plan flashPlan, slot int) (flashSlotResult, error) {
	result := flashSlotResult{Slot: slot, Status: "failed"}
	resp, err := sendBody(client, server, http.MethodPost, plan.pathFor(slot), plan.image, "application/octet-stream", controllerID, "")
	if err != nil {
		result.Detail = printable(err.Error())
		return result, err
	}
	if !okStatus(resp.Status) {
		result.Status = "refused"
		if resp.Status >= 500 {
			result.Status = "failed"
		}
		result.Detail = fmt.Sprintf("HTTP %d %s", resp.Status, resp.detail())
		return result, refusalError(fmt.Sprintf("Flashing slot %d", slot), resp)
	}
	var answer struct {
		Success    bool   `json:"success"`
		FinalPhase string `json:"finalPhase"`
	}
	if err := json.Unmarshal(resp.Body, &answer); err != nil {
		result.Detail = "the answer of the server could not be read"
		return result, newCliError(ExitServerError, fmt.Errorf("flashing slot %d: unexpected answer of the server: %w", slot, err))
	}
	if flashAnswerPhase.MatchString(answer.FinalPhase) {
		result.Phase = answer.FinalPhase
	}
	if !answer.Success {
		result.Detail = "the flashing service did not reach the final phase"
		return result, newCliError(ExitServerError, fmt.Errorf("slot %d was not flashed (the last phase was %q)", slot, result.Phase))
	}
	result.Status = "flashed"
	return result, nil
}

// cmdFlashAll implements `hydra-cli flash-all`.
func cmdFlashAll(w io.Writer, args []string) error {
	longTimeout := !explicitTimeout(args)
	server, timeout, controllerID, rest, err := parseControlArgs(args)
	if err != nil {
		return err
	}
	if longTimeout {
		timeout = defaultFlashTimeout
	}
	parsed, err := parseFlagSet("flash-all", rest,
		[]string{"--image", "--slots", "--tier", "--relay", "--hardware-id", "--image-version", "--signature", "--signature-file", "--downgrade-from", "--target", "--min-bootloader"},
		[]string{"--dry-run", "--yes", "--keep-going", "--json"})
	if err != nil {
		return err
	}
	if err := rejectExtraArgs("flash-all", parsed.Rest); err != nil {
		return err
	}
	jsonOutput, dryRun, confirmed, keepGoing := parsed.Bool("--json"), parsed.Bool("--dry-run"), parsed.Bool("--yes"), parsed.Bool("--keep-going")
	plan, err := buildFlashPlan(parsed)
	if err != nil {
		return err
	}
	if dryRun && confirmed {
		return newCliError(ExitUsageError, errors.New("--dry-run and --yes contradict each other: a dry run sends nothing"))
	}
	if !dryRun && !confirmed {
		return newCliError(ExitUsageError, fmt.Errorf("flash-all rewrites the firmware of %d board(s): add --yes to do it, or --dry-run to see the plan (nothing was sent)", len(plan.slots)))
	}
	results := make([]flashSlotResult, 0, len(plan.slots))
	if dryRun {
		for _, slot := range plan.slots {
			results = append(results, flashSlotResult{Slot: slot, Status: "planned", Detail: "POST " + plan.pathFor(slot)})
		}
		return printFlashAll(w, jsonOutput, true, server, plan, results, nil)
	}
	client := newClient(timeout)
	var firstFailure error
	stopped := false
	for _, slot := range plan.slots {
		if stopped {
			results = append(results, flashSlotResult{Slot: slot, Status: "not-tried", Detail: "an earlier slot failed"})
			continue
		}
		result, flashErr := flashOne(&client, server, controllerID, plan, slot)
		results = append(results, result)
		if flashErr != nil {
			if firstFailure == nil {
				firstFailure = flashErr
			}
			if !keepGoing {
				stopped = true
			}
		}
	}
	return printFlashAll(w, jsonOutput, false, server, plan, results, firstFailure)
}

// printFlashAll writes the plan or the outcome and returns the failure that decides the exit code.
func printFlashAll(w io.Writer, jsonOutput, dryRun bool, server string, plan flashPlan, results []flashSlotResult, failure error) error {
	if jsonOutput {
		rows := make([]map[string]any, 0, len(results))
		for _, r := range results {
			rows = append(rows, map[string]any{"slot": r.Slot, "status": r.Status, "phase": r.Phase, "detail": printable(r.Detail)})
		}
		if err := writeJSONResult(w, "flash-all", failure == nil, map[string]any{
			"dry_run": dryRun, "image_bytes": len(plan.image), "image_sha256": plan.imageSHA256, "slots": rows,
		}); err != nil {
			return err
		}
		return failure
	}
	if dryRun {
		fmt.Fprintf(w, "Dry run: nothing was sent. The image (%d bytes, sha256 %s) would go to %s:\n", len(plan.image), plan.imageSHA256, printable(server))
	}
	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "SLOT\tSTATUS\tPHASE\tDETAIL")
	for _, r := range results {
		fmt.Fprintf(table, "%d\t%s\t%s\t%s\n", r.Slot, r.Status, dash(r.Phase), dash(printable(r.Detail)))
	}
	if err := table.Flush(); err != nil {
		return err
	}
	if !dryRun {
		flashed := 0
		for _, r := range results {
			if r.Status == "flashed" {
				flashed++
			}
		}
		fmt.Fprintf(w, "%d of %d slot(s) flashed.\n", flashed, len(results))
	}
	return failure
}
