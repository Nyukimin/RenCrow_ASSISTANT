// Package verify implements the read-only RenCrow_ASSISTANT owner verifier.
package verify

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	Owner          = "RenCrow_ASSISTANT"
	ReceiptSchema  = "rencrow.check-receipt.v1"
	MaxEvidence    = 1 << 20
	MaxEvidenceAge = 5 * time.Minute
)

// CommandToCheck is the complete command allowlist owned by ASSISTANT. The
// check mapping is deliberate: a valid command cannot be reassigned to a
// different check by a manifest edit.
var CommandToCheck = map[string]string{
	"assistant-manual-notify-contract":              "assistant_manual_notify_contract",
	"assistant-source-build":                        "assistant_source_build",
	"assistant-deploy-identity-chain":               "assistant_deploy_identity_chain",
	"assistant-runtime-identity-lifecycle-security": "assistant_runtime_identity_lifecycle_security",
	"assistant-runtime-readiness":                   "assistant_runtime_readiness",
	"assistant-canonical-actor-e2e":                 "assistant_canonical_actor_e2e",
}

var (
	idPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	phaseSet  = map[string]bool{
		"any": true, "startup": true, "runtime": true, "deploy": true,
		"backup": true, "diagnostic": true,
	}
	actionSet = map[string]bool{
		"blocked": true, "rejected": true, "degraded": true, "notify": true,
	}
	costSet = map[string]bool{"low": true, "medium": true, "high": true}
)

// Options are the common owner-verifier CLI inputs.
type Options struct {
	ManifestPath string
	CheckID      string
	ObservedAt   string
	EvidenceDir  string
}

// Manifest is the v2 owner manifest representation.
type Manifest struct {
	SchemaVersion int     `json:"schema_version"`
	Purpose       string  `json:"purpose"`
	Phase         string  `json:"phase"`
	Checks        []Check `json:"checks"`
}

// Check is a manifest v2 check.
type Check struct {
	CheckID       string   `json:"check_id"`
	GuaranteeID   string   `json:"guarantee_id"`
	Owner         string   `json:"owner"`
	Purpose       string   `json:"purpose"`
	Target        string   `json:"target"`
	Phase         string   `json:"phase"`
	Consumer      string   `json:"consumer"`
	FailureAction string   `json:"failure_action"`
	Cost          string   `json:"cost"`
	SafetyGate    bool     `json:"safety_gate"`
	Coverage      []string `json:"coverage"`
	Executor      Executor `json:"executor"`
	ReceiptSchema string   `json:"receipt_schema"`
	Surfaces      []string `json:"surfaces,omitempty"`
}

// Executor identifies the owner CLI implementation in the manifest.
type Executor struct {
	Kind      string `json:"kind"`
	CommandID string `json:"command_id"`
}

// Receipt is the common v1 owner-check receipt. It intentionally contains no
// evidence payload, credentials, or arbitrary command output.
type Receipt struct {
	SchemaVersion   int      `json:"schema_version"`
	ReceiptSchema   string   `json:"receipt_schema"`
	CheckID         string   `json:"check_id"`
	GuaranteeID     string   `json:"guarantee_id"`
	Owner           string   `json:"owner"`
	Status          string   `json:"status"`
	ObservedAt      string   `json:"observed_at"`
	RouteOrTarget   string   `json:"route_or_target"`
	EvidenceRefs    []string `json:"evidence_refs"`
	FailureBoundary string   `json:"failure_boundary"`
}

// InputError identifies a CLI or manifest contract error. These errors do not
// produce an aggregateable receipt and map to exit status 2.
type InputError struct{ Err error }

func (e *InputError) Error() string { return e.Err.Error() }
func (e *InputError) Unwrap() error { return e.Err }

// Run parses the owner verifier command and writes either one receipt or a
// concise CLI error. It never executes a user-supplied shell command.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		fmt.Fprintln(stderr, "usage: rencrow-assistant-verify run --manifest <path> --check-id <id> --observed-at <RFC3339-UTC> --evidence-dir <dir>")
		return 2
	}
	flags := flag.NewFlagSet("rencrow-assistant-verify run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "", "owner manifest path")
	checkID := flags.String("check-id", "", "declared check ID")
	observedAt := flags.String("observed-at", "", "RFC3339 UTC observation time")
	evidenceDir := flags.String("evidence-dir", "", "bounded evidence directory")
	if err := flags.Parse(args[1:]); err != nil {
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "[NG] unexpected positional arguments")
		return 2
	}
	receipt, code, err := Execute(Options{
		ManifestPath: *manifestPath,
		CheckID:      *checkID,
		ObservedAt:   *observedAt,
		EvidenceDir:  *evidenceDir,
	})
	if err != nil {
		fmt.Fprintf(stderr, "[NG] %v\n", err)
		return code
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(receipt); err != nil {
		fmt.Fprintf(stderr, "[NG] encode receipt: %v\n", err)
		return 2
	}
	return code
}

// Execute validates the manifest and evaluates one selected check against
// evidence already present in the explicit evidence directory.
func Execute(options Options) (Receipt, int, error) {
	manifest, err := LoadManifest(options.ManifestPath)
	if err != nil {
		return Receipt{}, 2, &InputError{Err: err}
	}
	if strings.TrimSpace(options.CheckID) == "" {
		return Receipt{}, 2, &InputError{Err: errors.New("--check-id is required")}
	}
	observedAt, err := parseUTC(options.ObservedAt)
	if err != nil {
		return Receipt{}, 2, &InputError{Err: fmt.Errorf("--observed-at: %w", err)}
	}
	if strings.TrimSpace(options.EvidenceDir) == "" {
		return Receipt{}, 2, &InputError{Err: errors.New("--evidence-dir is required")}
	}
	check, ok := findCheck(manifest.Checks, options.CheckID)
	if !ok {
		return Receipt{}, 2, &InputError{Err: fmt.Errorf("unknown check_id %q", options.CheckID)}
	}
	base := Receipt{
		SchemaVersion:   1,
		ReceiptSchema:   ReceiptSchema,
		CheckID:         check.CheckID,
		GuaranteeID:     check.GuaranteeID,
		Owner:           Owner,
		ObservedAt:      observedAt.Format(time.RFC3339Nano),
		RouteOrTarget:   check.Target,
		EvidenceRefs:    []string{},
		FailureBoundary: "",
	}

	root, rootErr := boundedDirectory(options.EvidenceDir)
	if rootErr != nil {
		base.Status = "blocked"
		base.FailureBoundary = "required evidence directory is unavailable"
		return base, 20, nil
	}
	evidencePath, found, findErr := findEvidence(root, check)
	if findErr != nil {
		base.Status = "unverified"
		base.FailureBoundary = "evidence could not be read"
		return base, 30, nil
	}
	if !found {
		base.Status = "blocked"
		base.FailureBoundary = "required evidence is missing"
		return base, 20, nil
	}
	evidence, evidenceRefs, evidenceErr := readEvidence(root, evidencePath, check, observedAt)
	base.EvidenceRefs = evidenceRefs
	if evidenceErr != nil {
		base.Status = "unverified"
		base.FailureBoundary = "evidence is malformed or does not prove the declared check"
		return base, 30, nil
	}
	base.RouteOrTarget = evidence.route
	switch evidence.status {
	case "passed":
		base.Status = "passed"
		return base, 0, nil
	case "failed":
		base.Status = "failed"
		base.FailureBoundary = "owner evidence reported failed"
		return base, 10, nil
	case "blocked":
		base.Status = "blocked"
		base.FailureBoundary = "owner evidence reported blocked"
		return base, 20, nil
	case "not_applicable":
		base.Status = "not_applicable"
		return base, 0, nil
	default:
		base.Status = "unverified"
		base.FailureBoundary = "owner evidence status is not a valid receipt status"
		return base, 30, nil
	}
}

// LoadManifest strictly parses and validates all checks before any selected
// check can be evaluated. This prevents an unknown command elsewhere in the
// manifest from being silently skipped.
func LoadManifest(path string) (Manifest, error) {
	if strings.TrimSpace(path) == "" {
		return Manifest{}, errors.New("--manifest is required")
	}
	var manifest Manifest
	if err := decodeJSONFile(path, &manifest, MaxEvidence); err != nil {
		return Manifest{}, fmt.Errorf("load manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return Manifest{}, fmt.Errorf("manifest schema_version must be 2")
	}
	if strings.TrimSpace(manifest.Purpose) == "" || strings.TrimSpace(manifest.Phase) == "" {
		return Manifest{}, errors.New("manifest purpose and phase are required")
	}
	if !phaseSet[manifest.Phase] {
		return Manifest{}, fmt.Errorf("manifest has invalid phase %q", manifest.Phase)
	}
	if len(manifest.Checks) == 0 {
		return Manifest{}, errors.New("manifest checks must not be empty")
	}
	seenChecks := make(map[string]bool, len(manifest.Checks))
	seenCommands := make(map[string]bool, len(manifest.Checks))
	for _, check := range manifest.Checks {
		if err := validateCheck(check, seenChecks, seenCommands); err != nil {
			return Manifest{}, err
		}
		seenChecks[check.CheckID] = true
		seenCommands[check.Executor.CommandID] = true
	}
	return manifest, nil
}

func validateCheck(check Check, seenChecks, seenCommands map[string]bool) error {
	if !idPattern.MatchString(check.CheckID) {
		return fmt.Errorf("invalid check_id %q", check.CheckID)
	}
	if seenChecks[check.CheckID] {
		return fmt.Errorf("duplicate check_id %q", check.CheckID)
	}
	for name, value := range map[string]string{
		"guarantee_id": check.GuaranteeID, "owner": check.Owner, "purpose": check.Purpose,
		"target": check.Target, "phase": check.Phase, "consumer": check.Consumer,
		"failure_action": check.FailureAction, "cost": check.Cost,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("check %q: %s is required", check.CheckID, name)
		}
	}
	if check.Owner != Owner {
		return fmt.Errorf("check %q owner must be %q", check.CheckID, Owner)
	}
	if !phaseSet[check.Phase] {
		return fmt.Errorf("check %q has invalid phase %q", check.CheckID, check.Phase)
	}
	if !actionSet[check.FailureAction] {
		return fmt.Errorf("check %q has invalid failure_action %q", check.CheckID, check.FailureAction)
	}
	if !costSet[check.Cost] {
		return fmt.Errorf("check %q has invalid cost %q", check.CheckID, check.Cost)
	}
	if len(check.Coverage) == 0 {
		return fmt.Errorf("check %q coverage must not be empty", check.CheckID)
	}
	for _, coverage := range check.Coverage {
		if strings.TrimSpace(coverage) == "" {
			return fmt.Errorf("check %q contains empty coverage", check.CheckID)
		}
		if coverage == "security_exposure" && !check.SafetyGate {
			return fmt.Errorf("check %q security_exposure requires safety_gate=true", check.CheckID)
		}
	}
	if check.Executor.Kind != "owner_cli" {
		return fmt.Errorf("check %q executor.kind must be owner_cli", check.CheckID)
	}
	command := strings.TrimSpace(check.Executor.CommandID)
	if command == "" {
		return fmt.Errorf("check %q executor.command_id is required", check.CheckID)
	}
	if seenCommands[command] {
		return fmt.Errorf("duplicate executor.command_id %q", command)
	}
	expectedCheck, ok := CommandToCheck[command]
	if !ok {
		return fmt.Errorf("unknown executor.command_id %q", command)
	}
	if expectedCheck != check.CheckID {
		return fmt.Errorf("executor.command_id %q is only valid for check_id %q", command, expectedCheck)
	}
	if check.ReceiptSchema != ReceiptSchema {
		return fmt.Errorf("check %q receipt_schema must be %q", check.CheckID, ReceiptSchema)
	}
	return nil
}

func findCheck(checks []Check, id string) (Check, bool) {
	for _, check := range checks {
		if check.CheckID == id {
			return check, true
		}
	}
	return Check{}, false
}

func parseUTC(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return time.Time{}, errors.New("--observed-at is required")
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, errors.New("must be RFC3339 UTC")
	}
	_, offset := parsed.Zone()
	if offset != 0 {
		return time.Time{}, errors.New("must be RFC3339 UTC")
	}
	return parsed.UTC(), nil
}

func boundedDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("evidence path is not a directory")
	}
	return absolute, nil
}

func findEvidence(root string, check Check) (string, bool, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	names := []string{check.CheckID + ".json", check.Executor.CommandID + ".json", "evidence.json"}
	seen := make(map[string]bool)
	var malformedCandidate string
	for _, name := range names {
		seen[name] = true
		if candidate, ok := evidenceEntry(entries, name); ok {
			if matchesEvidence(candidate, root, check) {
				return candidate, true, nil
			}
			if malformedCandidate == "" {
				malformedCandidate = candidate
			}
		}
	}
	for _, entry := range entries {
		if seen[entry.Name()] || entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		candidate := entry.Name()
		if matchesEvidence(candidate, root, check) {
			return candidate, true, nil
		}
	}
	if malformedCandidate != "" {
		return malformedCandidate, true, nil
	}
	return "", false, nil
}

func evidenceEntry(entries []os.DirEntry, name string) (string, bool) {
	for _, entry := range entries {
		if entry.Name() == name && !entry.IsDir() {
			return entry.Name(), true
		}
	}
	return "", false
}

func matchesEvidence(candidate, root string, check Check) bool {
	path := filepath.Join(root, candidate)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return false
	}
	var document map[string]json.RawMessage
	if err := decodeJSONFile(path, &document, MaxEvidence); err != nil {
		return false
	}
	return rawString(document, "check_id") == check.CheckID &&
		rawString(document, "command_id") == check.Executor.CommandID &&
		rawString(document, "owner") == Owner
}

type evidenceDocument struct {
	status string
	route  string
	refs   []string
}

func readEvidence(root, relativePath string, check Check, observedAt time.Time) (evidenceDocument, []string, error) {
	path := filepath.Join(root, relativePath)
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, errors.New("evidence file is not a bounded regular file")
	}
	var document map[string]json.RawMessage
	if err := decodeJSONFile(path, &document, MaxEvidence); err != nil {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, err
	}
	if rawString(document, "check_id") != check.CheckID ||
		rawString(document, "command_id") != check.Executor.CommandID ||
		rawString(document, "owner") != Owner {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, errors.New("evidence identity mismatch")
	}
	status := strings.TrimSpace(rawString(document, "status"))
	if status == "" {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, errors.New("evidence status is required")
	}
	route := firstString(document, "route_or_target", "route", "target")
	if route == "" || !safeRoute(route) {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, errors.New("safe route_or_target is required")
	}
	evidenceObserved, err := parseUTC(rawString(document, "observed_at"))
	if err != nil || evidenceObserved.After(observedAt) || evidenceObserved.Before(observedAt.Add(-MaxEvidenceAge)) {
		return evidenceDocument{}, []string{relativeRef(relativePath)}, errors.New("evidence observed_at is invalid, stale, or in the future")
	}
	refs, err := boundedRefs(root, relativePath, document)
	if err != nil {
		return evidenceDocument{}, refs, err
	}
	if status == "passed" && !proofFor(check.Executor.CommandID, document) {
		return evidenceDocument{}, refs, errors.New("required proof is missing")
	}
	if status == "not_applicable" && firstString(document, "reason", "rationale") == "" {
		return evidenceDocument{}, refs, errors.New("not_applicable reason is required")
	}
	return evidenceDocument{status: status, route: route, refs: refs}, refs, nil
}

func boundedRefs(root, evidencePath string, document map[string]json.RawMessage) ([]string, error) {
	refs := []string{relativeRef(evidencePath)}
	seen := map[string]bool{refs[0]: true}
	var declared []string
	if raw, ok := document["evidence_refs"]; ok {
		if err := json.Unmarshal(raw, &declared); err != nil {
			return refs, errors.New("evidence_refs must be an array of strings")
		}
	}
	for _, value := range declared {
		relative, err := normalizeRef(value)
		if err != nil {
			return refs, err
		}
		if seen[relative] {
			continue
		}
		candidate := filepath.Join(root, filepath.FromSlash(relative[len("relative:"):]))
		info, err := os.Lstat(candidate)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return refs, errors.New("evidence reference is not a bounded regular file")
		}
		seen[relative] = true
		refs = append(refs, relative)
	}
	sort.Strings(refs)
	return refs, nil
}

func normalizeRef(value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "relative:") {
		value = strings.TrimPrefix(value, "relative:")
	}
	// Treat both separators as path separators even when a receipt produced on
	// another OS is being verified locally.
	value = strings.ReplaceAll(value, "\\", "/")
	if value == "" || filepath.IsAbs(value) || filepath.VolumeName(value) != "" ||
		(len(value) >= 2 && value[1] == ':') {
		return "", errors.New("evidence reference must be relative")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+filepathSeparator) {
		return "", errors.New("evidence reference escapes evidence directory")
	}
	return "relative:" + filepath.ToSlash(clean), nil
}

// Keep the escape check platform independent.
const filepathSeparator = string(os.PathSeparator)

func relativeRef(value string) string {
	clean, err := normalizeRef(value)
	if err != nil {
		return "relative:evidence.json"
	}
	return clean
}

func safeRoute(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n") {
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"token", "api_key", "apikey", "secret", "password", "authorization", "bearer "} {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	return !strings.Contains(value, "@")
}

func proofFor(command string, document map[string]json.RawMessage) bool {
	required := []string{}
	switch command {
	case "assistant-deploy-identity-chain":
		required = []string{"source_revision", "artifact_identity", "publication_identity", "request_receipt"}
	case "assistant-runtime-identity-lifecycle-security":
		required = []string{"runtime_identity", "lifecycle", "security_exposure", "listener_identity", "request_receipt"}
	case "assistant-canonical-actor-e2e":
		required = []string{"actor", "authenticated", "canonical_route", "request_receipt"}
	case "assistant-manual-notify-contract":
		required = []string{"request_receipt"}
	case "assistant-source-build":
		required = []string{"artifact_identity"}
	case "assistant-runtime-readiness":
		required = []string{"readiness"}
	default:
		return false
	}
	for _, key := range required {
		if key == "request_receipt" {
			if !proofAny(document, "request_receipt", "receipt_trace", "canonical_request_receipt") {
				return false
			}
			continue
		}
		if key == "listener_identity" {
			if !proofAny(document, "listener_identity", "service_listener", "listener", "service_identity") {
				return false
			}
			continue
		}
		if !proofValue(document, key) {
			return false
		}
	}
	if command == "assistant-canonical-actor-e2e" {
		raw, ok := proofRaw(document, "authenticated")
		if !ok {
			return false
		}
		var authenticated bool
		if err := json.Unmarshal(raw, &authenticated); err != nil || !authenticated {
			return false
		}
	}
	return true
}

func proofAny(document map[string]json.RawMessage, keys ...string) bool {
	for _, key := range keys {
		if proofValue(document, key) {
			return true
		}
	}
	return false
}

func proofValue(document map[string]json.RawMessage, key string) bool {
	raw, ok := proofRaw(document, key)
	if !ok {
		return false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return false
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) != ""
	case bool:
		return typed
	case nil:
		return false
	default:
		return true
	}
}

func proofRaw(document map[string]json.RawMessage, key string) (json.RawMessage, bool) {
	if raw, ok := document[key]; ok {
		return raw, true
	}
	proof, ok := document["proof"]
	if !ok {
		return nil, false
	}
	var nested map[string]json.RawMessage
	if json.Unmarshal(proof, &nested) != nil {
		return nil, false
	}
	raw, ok := nested[key]
	return raw, ok
}

func firstString(document map[string]json.RawMessage, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(rawString(document, key)); value != "" {
			return value
		}
	}
	return ""
}

func rawString(document map[string]json.RawMessage, key string) string {
	raw, ok := document[key]
	if !ok {
		return ""
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	return value
}

func decodeJSONFile(path string, value any, limit int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return errors.New("JSON file exceeds the bounded size or is not regular")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, limit+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("JSON must contain exactly one value")
		}
		return err
	}
	return nil
}
