package service_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/unclestep/Rogue/internal/domain/service"
)

// NewONNXPolicy with an empty path must reject immediately, before any
// attempt to touch the ONNX Runtime C library. This is the fast-path that
// providePursuerPolicy relies on to skip ONNX entirely when no model is
// configured, and it must never drag the library into the process.
func TestNewONNXPolicyEmptyPathRejected(t *testing.T) {
	_, err := service.NewONNXPolicy("")
	if err == nil {
		t.Fatalf("expected error for empty model path, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error message, got %q", err)
	}
}

// Same guarantee for a path that resolves to a missing file: the stat check
// in NewONNXPolicy must fire before the C library touches anything. This
// keeps the soft-fail contract honest in CI environments where
// libonnxruntime.so is not installed.
func TestNewONNXPolicyMissingFileRejected(t *testing.T) {
	_, err := service.NewONNXPolicy("/nonexistent/path/pursuer.onnx")
	if err == nil {
		t.Fatalf("expected error for missing model file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error message, got %q", err)
	}
}

// Integration test: load the committed testdata model and run one inference.
// Skipped when the ONNX Runtime shared library is unavailable (typical CI),
// so the test gate aligns with the soft-dependency story — the game runs
// without ONNX, and so does `go test ./...`.
//
// gen_tiny_onnx.py biases the logits so argmax == 0 regardless of input.
func TestONNXPolicyPredictOnTestdataModel(t *testing.T) {
	modelPath, err := filepath.Abs(
		"../../internal/domain/service/testdata/pursuer_tiny.onnx")
	if err != nil {
		t.Fatalf("resolving model path: %v", err)
	}

	policy, err := service.NewONNXPolicy(modelPath)
	if err != nil {
		// Expected on hosts without libonnxruntime.so — keep CI green.
		t.Skipf("ONNX Runtime unavailable, skipping integration test: %v", err)
	}
	defer policy.Destroy()

	obs := make([]float32, service.ObservationSize)
	action := policy.Predict(nil, nil, obs)

	if action != service.ActionUp {
		t.Errorf("expected ActionUp (argmax=0 baked into testdata model), got %d",
			action)
	}
}

// Calling Predict with a wrong-sized observation must not crash — ONNXPolicy
// should log and return ActionWait. Protects against future refactors that
// change ObservationSize without updating the Python model in lockstep.
func TestONNXPolicyPredictWrongObsSizeYieldsWait(t *testing.T) {
	modelPath, err := filepath.Abs(
		"../../internal/domain/service/testdata/pursuer_tiny.onnx")
	if err != nil {
		t.Fatalf("resolving model path: %v", err)
	}

	policy, err := service.NewONNXPolicy(modelPath)
	if err != nil {
		t.Skipf("ONNX Runtime unavailable, skipping: %v", err)
	}
	defer policy.Destroy()

	action := policy.Predict(nil, nil, []float32{0, 0, 0})
	if action != service.ActionWait {
		t.Errorf("expected ActionWait on wrong-sized obs, got %d", action)
	}
}
