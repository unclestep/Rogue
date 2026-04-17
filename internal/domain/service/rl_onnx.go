package service

import (
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	ort "github.com/yalue/onnxruntime_go"
	"github.com/unclestep/Rogue/internal/domain/model"
)

// ONNX input/output names produced by rl/pursuer_training.ipynb (PR 4).
// Keep in lock-step with the Python exporter.
const (
	onnxInputName  = "input"
	onnxOutputName = "logits"
)

// OnnxLibEnv names the environment variable that overrides the shared-library
// lookup. When unset, onnxruntime_go falls back to its platform default (OS
// library search path). Leaving it empty is fine — we'll only try to load
// when the caller explicitly supplies a model path.
const OnnxLibEnv = "ROGUE_ONNX_LIB_PATH"

var (
	ortInitOnce sync.Once
	ortInitErr  error
)

// initONNXRuntime performs the one-time global initialisation of the ONNX
// Runtime C library. It is safe to call from multiple goroutines; the first
// caller does the work, subsequent callers reuse the stored error (if any).
// Subsequent Destroy is left to the process lifetime — the library is fine
// with a single init + leak on exit, which is the norm for long-lived servers.
func initONNXRuntime() error {
	ortInitOnce.Do(func() {
		if p := os.Getenv(OnnxLibEnv); p != "" {
			ort.SetSharedLibraryPath(p)
		}
		ortInitErr = ort.InitializeEnvironment()
	})
	return ortInitErr
}

// ONNXPolicy runs inference on a trained Pursuer model.
// A single session is shared across all Pursuers in the game (all monsters
// consult the same brain); concurrent Predict calls are serialised via mu
// because DynamicAdvancedSession.Run is not thread-safe.
type ONNXPolicy struct {
	mu      sync.Mutex
	session *ort.DynamicAdvancedSession
}

// NewONNXPolicy loads a model from disk and readies an inference session.
// Returns an error if:
//   - the ONNX Runtime shared library is missing (CI without libonnxruntime),
//   - modelPath does not exist or is unreadable,
//   - the model is structurally invalid.
//
// Callers are expected to fall back to FallbackPolicy on any error — the
// game must keep running even without a trained model.
func NewONNXPolicy(modelPath string) (*ONNXPolicy, error) {
	if modelPath == "" {
		return nil, errors.New("ONNX model path is empty")
	}
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("ONNX model not found at %q: %w", modelPath, err)
	}
	if err := initONNXRuntime(); err != nil {
		return nil, fmt.Errorf("ONNX Runtime init failed: %w", err)
	}

	session, err := ort.NewDynamicAdvancedSession(modelPath,
		[]string{onnxInputName}, []string{onnxOutputName}, nil)
	if err != nil {
		return nil, fmt.Errorf("ONNX session creation failed: %w", err)
	}

	return &ONNXPolicy{session: session}, nil
}

// Predict runs the policy network on the given observation and returns the
// argmax action. The observation length must equal ObservationSize.
// On any inference error we log the cause once and return ActionWait so the
// game keeps moving even if the model misbehaves.
func (p *ONNXPolicy) Predict(_ *model.SessionContext, _ *model.Actor, obs []float32) int {
	if len(obs) != ObservationSize {
		log.Printf("[ERROR] ONNXPolicy.Predict: obs length=%d, want %d", len(obs), ObservationSize)
		return ActionWait
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	input, err := ort.NewTensor(ort.NewShape(1, int64(ObservationSize)), obs)
	if err != nil {
		log.Printf("[ERROR] ONNXPolicy.Predict: input tensor: %v", err)
		return ActionWait
	}
	defer input.Destroy()

	output, err := ort.NewEmptyTensor[float32](ort.NewShape(1, ActionCount))
	if err != nil {
		log.Printf("[ERROR] ONNXPolicy.Predict: output tensor: %v", err)
		return ActionWait
	}
	defer output.Destroy()

	if err := p.session.Run([]ort.Value{input}, []ort.Value{output}); err != nil {
		log.Printf("[ERROR] ONNXPolicy.Predict: Run: %v", err)
		return ActionWait
	}

	return argmaxAction(output.GetData())
}

// Destroy releases the underlying C session. Callers should invoke it when
// tearing down a training/testing fixture; the production singleton leaks
// intentionally on process exit.
func (p *ONNXPolicy) Destroy() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session == nil {
		return nil
	}
	err := p.session.Destroy()
	p.session = nil
	return err
}

// argmaxAction returns the index of the largest logit, restricted to the
// defined action space. Short-circuits on malformed output.
func argmaxAction(logits []float32) int {
	if len(logits) == 0 {
		return ActionWait
	}
	best := 0
	for i := 1; i < len(logits) && i < ActionCount; i++ {
		if logits[i] > logits[best] {
			best = i
		}
	}
	return best
}
