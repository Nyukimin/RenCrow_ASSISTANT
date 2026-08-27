package verify

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testObservedAt = "2026-08-27T00:00:01Z"

func TestLoadManifestCurrentConfigCoversEveryAllowlistedCommand(t *testing.T) {
	manifest, err := LoadManifest(filepath.Join("..", "..", "config", "checks", "runtime.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Checks) != len(CommandToCheck) {
		t.Fatalf("checks=%d allowlist=%d", len(manifest.Checks), len(CommandToCheck))
	}
	for _, check := range manifest.Checks {
		if got := CommandToCheck[check.Executor.CommandID]; got != check.CheckID {
			t.Fatalf("command %q mapped to %q, check=%q", check.Executor.CommandID, got, check.CheckID)
		}
	}
}

func TestExecuteEveryCommandIDWithSuccessEvidence(t *testing.T) {
	manifest := validManifest()
	manifestPath := writeManifest(t, manifest)
	evidenceDir := t.TempDir()
	for _, check := range manifest.Checks {
		writeEvidence(t, evidenceDir, check, map[string]any{
			"status":          "passed",
			"observed_at":     testObservedAt,
			"route_or_target": "local owner route",
			"proof":           proofForTest(check.Executor.CommandID),
		})
		receipt, code, err := Execute(Options{
			ManifestPath: manifestPath,
			CheckID:      check.CheckID,
			ObservedAt:   testObservedAt,
			EvidenceDir:  evidenceDir,
		})
		if err != nil {
			t.Fatalf("%s: %v", check.CheckID, err)
		}
		if code != 0 || receipt.Status != "passed" {
			t.Fatalf("%s: code=%d receipt=%#v", check.CheckID, code, receipt)
		}
		assertReceiptContract(t, receipt, check)
	}
}

func TestExecuteMissingEvidenceReturnsBlockedReceipt(t *testing.T) {
	manifest := validManifest()
	receipt, code, err := Execute(Options{
		ManifestPath: writeManifest(t, manifest),
		CheckID:      manifest.Checks[0].CheckID,
		ObservedAt:   testObservedAt,
		EvidenceDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 20 || receipt.Status != "blocked" {
		t.Fatalf("code=%d receipt=%#v", code, receipt)
	}
	if receipt.FailureBoundary == "" || len(receipt.EvidenceRefs) != 0 {
		t.Fatalf("receipt=%#v", receipt)
	}
}

func TestExecuteMalformedEvidenceReturnsUnverifiedReceipt(t *testing.T) {
	manifest := validManifest()
	evidenceDir := t.TempDir()
	check := manifest.Checks[0]
	if err := os.WriteFile(filepath.Join(evidenceDir, check.CheckID+".json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	receipt, code, err := Execute(Options{
		ManifestPath: writeManifest(t, manifest),
		CheckID:      check.CheckID,
		ObservedAt:   testObservedAt,
		EvidenceDir:  evidenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 30 || receipt.Status != "unverified" {
		t.Fatalf("code=%d receipt=%#v", code, receipt)
	}
}

func TestExecuteRejectsEvidenceOlderThanFiveMinutes(t *testing.T) {
	manifest := validManifest()
	evidenceDir := t.TempDir()
	check := manifest.Checks[0]
	writeEvidence(t, evidenceDir, check, map[string]any{
		"status":          "passed",
		"observed_at":     "2026-08-26T23:54:59Z",
		"route_or_target": "local owner route",
		"proof":           proofForTest(check.Executor.CommandID),
	})
	receipt, code, err := Execute(Options{
		ManifestPath: writeManifest(t, manifest),
		CheckID:      check.CheckID,
		ObservedAt:   testObservedAt,
		EvidenceDir:  evidenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 30 || receipt.Status != "unverified" {
		t.Fatalf("code=%d receipt=%#v", code, receipt)
	}
}

func TestExecuteAcceptsEvidenceAtFiveMinuteFreshnessBoundary(t *testing.T) {
	manifest := validManifest()
	evidenceDir := t.TempDir()
	check := manifest.Checks[0]
	writeEvidence(t, evidenceDir, check, map[string]any{
		"status":          "passed",
		"observed_at":     "2026-08-26T23:55:01Z",
		"route_or_target": "local owner route",
		"proof":           proofForTest(check.Executor.CommandID),
	})
	receipt, code, err := Execute(Options{
		ManifestPath: writeManifest(t, manifest),
		CheckID:      check.CheckID,
		ObservedAt:   testObservedAt,
		EvidenceDir:  evidenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || receipt.Status != "passed" {
		t.Fatalf("code=%d receipt=%#v", code, receipt)
	}
}

func TestExecuteRejectsEvidenceReferenceOutsideDirectory(t *testing.T) {
	manifest := validManifest()
	evidenceDir := t.TempDir()
	check := manifest.Checks[0]
	writeEvidence(t, evidenceDir, check, map[string]any{
		"status":          "passed",
		"observed_at":     testObservedAt,
		"route_or_target": "local owner route",
		"proof":           proofForTest(check.Executor.CommandID),
		"evidence_refs":   []string{"../outside.json"},
	})
	receipt, code, err := Execute(Options{
		ManifestPath: writeManifest(t, manifest),
		CheckID:      check.CheckID,
		ObservedAt:   testObservedAt,
		EvidenceDir:  evidenceDir,
	})
	if err != nil {
		t.Fatal(err)
	}
	if code != 30 || receipt.Status != "unverified" {
		t.Fatalf("code=%d receipt=%#v", code, receipt)
	}
}

func TestLoadManifestRejectsUnknownCommandBeforeSelectedCheck(t *testing.T) {
	manifest := validManifest()
	manifest.Checks[1].Executor.CommandID = "assistant-unknown-command"
	if _, err := LoadManifest(writeManifest(t, manifest)); err == nil || !strings.Contains(err.Error(), "unknown executor.command_id") {
		t.Fatalf("err=%v", err)
	}
}

func TestLoadManifestRejectsMalformedSchemaAndOwnerMismatch(t *testing.T) {
	manifest := validManifest()
	manifest.SchemaVersion = 1
	if _, err := LoadManifest(writeManifest(t, manifest)); err == nil {
		t.Fatal("schema version must be rejected")
	}
	manifest = validManifest()
	manifest.Checks[0].Owner = "RenCrow_CORE"
	if _, err := LoadManifest(writeManifest(t, manifest)); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("err=%v", err)
	}
}

func TestExecuteRejectsUnknownSelectedCheckAndRunEmitsJSONReceipt(t *testing.T) {
	manifest := validManifest()
	manifestPath := writeManifest(t, manifest)
	receipt, code, err := Execute(Options{
		ManifestPath: manifestPath,
		CheckID:      "unknown-check",
		ObservedAt:   testObservedAt,
		EvidenceDir:  t.TempDir(),
	})
	if err == nil || code != 2 || receipt.SchemaVersion != 0 || receipt.CheckID != "" {
		t.Fatalf("receipt=%#v code=%d err=%v", receipt, code, err)
	}

	var stdout, stderr bytes.Buffer
	code = Run([]string{
		"run", "--manifest", manifestPath,
		"--check-id", manifest.Checks[0].CheckID,
		"--observed-at", testObservedAt,
		"--evidence-dir", t.TempDir(),
	}, &stdout, &stderr)
	if code != 20 || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	var parsed Receipt
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Status != "blocked" || parsed.ReceiptSchema != ReceiptSchema {
		t.Fatalf("receipt=%#v", parsed)
	}
}

func TestReceiptNeverContainsEvidenceSecretValues(t *testing.T) {
	manifest := validManifest()
	evidenceDir := t.TempDir()
	check := manifest.Checks[0]
	writeEvidence(t, evidenceDir, check, map[string]any{
		"status":          "passed",
		"observed_at":     testObservedAt,
		"route_or_target": "local owner route",
		"proof":           proofForTest(check.Executor.CommandID),
		"private_token":   "must-not-be-relayed",
	})
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"run", "--manifest", writeManifest(t, manifest),
		"--check-id", check.CheckID,
		"--observed-at", testObservedAt,
		"--evidence-dir", evidenceDir,
	}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if strings.Contains(stdout.String(), "must-not-be-relayed") || strings.Contains(stdout.String(), "private_token") {
		t.Fatalf("secret leaked in receipt: %q", stdout.String())
	}
}

func validManifest() Manifest {
	checks := make([]Check, 0, len(CommandToCheck))
	commands := make([]string, 0, len(CommandToCheck))
	for command := range CommandToCheck {
		commands = append(commands, command)
	}
	sortStrings(commands)
	for _, command := range commands {
		checks = append(checks, Check{
			CheckID:       CommandToCheck[command],
			GuaranteeID:   CommandToCheck[command] + "_verified",
			Owner:         Owner,
			Purpose:       "test check",
			Target:        "local owner route",
			Phase:         "runtime",
			Consumer:      "test consumer",
			FailureAction: "blocked",
			Cost:          "low",
			Coverage:      []string{"readiness"},
			Executor:      Executor{Kind: "owner_cli", CommandID: command},
			ReceiptSchema: ReceiptSchema,
		})
	}
	return Manifest{SchemaVersion: 2, Purpose: "operational_status", Phase: "runtime", Checks: checks}
}

func proofForTest(command string) map[string]any {
	proof := map[string]any{}
	switch command {
	case "assistant-manual-notify-contract":
		proof["request_receipt"] = "receipt"
	case "assistant-source-build":
		proof["artifact_identity"] = "artifact"
	case "assistant-deploy-identity-chain":
		proof["source_revision"] = "source"
		proof["artifact_identity"] = "artifact"
		proof["publication_identity"] = "publication"
		proof["request_receipt"] = "receipt"
	case "assistant-runtime-identity-lifecycle-security":
		proof["runtime_identity"] = "runtime"
		proof["lifecycle"] = "single process"
		proof["security_exposure"] = "protected"
		proof["listener_identity"] = "listener"
		proof["request_receipt"] = "receipt"
	case "assistant-runtime-readiness":
		proof["readiness"] = true
	case "assistant-canonical-actor-e2e":
		proof["actor"] = "authenticated user"
		proof["authenticated"] = true
		proof["canonical_route"] = "assistant delivery route"
		proof["request_receipt"] = "receipt trace"
	}
	return proof
}

func writeManifest(t *testing.T, manifest Manifest) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "runtime.json")
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEvidence(t *testing.T, directory string, check Check, fields map[string]any) {
	t.Helper()
	fields["check_id"] = check.CheckID
	fields["command_id"] = check.Executor.CommandID
	fields["owner"] = Owner
	data, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, check.CheckID+".json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertReceiptContract(t *testing.T, receipt Receipt, check Check) {
	t.Helper()
	if receipt.SchemaVersion != 1 || receipt.ReceiptSchema != ReceiptSchema ||
		receipt.CheckID != check.CheckID || receipt.GuaranteeID != check.GuaranteeID ||
		receipt.Owner != Owner || receipt.ObservedAt != testObservedAt ||
		receipt.RouteOrTarget == "" || len(receipt.EvidenceRefs) == 0 || receipt.FailureBoundary != "" {
		t.Fatalf("receipt=%#v", receipt)
	}
	for _, ref := range receipt.EvidenceRefs {
		if !strings.HasPrefix(ref, "relative:") || strings.Contains(ref, "..") {
			t.Fatalf("unsafe evidence ref %q", ref)
		}
	}
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
