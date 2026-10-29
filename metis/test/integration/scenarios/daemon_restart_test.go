/*
Copyright 2026 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package scenarios

import (
	"fmt"
	"slices"
	"testing"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/metis/pkg/store"
	"k8s.io/metis/test/integration"
)

// TestDaemonRestartRehydration verifies that stopping the daemon and starting a
// fresh instance against the same database rehydrates state, reconciles with
// NNC without spurious scale-ups, and continues serving pod allocations.
func TestDaemonRestartRehydration(t *testing.T) {
	f := integration.NewFixture(t)

	// Pre-restart capacity expansion: allocate initial pods and trigger a dynamic
	// block.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-rehydrate-init-%d", i))
	}
	_, dynamicCIDR := f.AllocatePod("pod-rehydrate-overflow")
	f.Eventually("dynamic block is active and allocations stabilized before restart", func() (bool, string) {
		nnc := f.NNC()
		var unmet []string
		if state := f.BlockState(dynamicCIDR); state != store.StateReady {
			unmet = append(unmet, fmt.Sprintf("DB block state: got %s, want %s", state, store.StateReady))
		}
		if ranges := f.GCEAliasRanges(); len(ranges) != 2 {
			unmet = append(unmet, fmt.Sprintf("GCE alias ranges count: got %d, want 2", len(ranges)))
		}
		if len(nnc.Spec.Allocations) == 0 {
			unmet = append(unmet, "NNC spec allocations is empty")
		} else if nnc.Spec.Allocations[0].Pods != 32 {
			unmet = append(unmet, fmt.Sprintf("NNC spec allocations pods: got %d, want 32", nnc.Spec.Allocations[0].Pods))
		}
		return integration.FormatUnmet(unmet)
	})

	allocationsBeforeRestart := slices.Clone(f.NNC().Spec.Allocations)

	// Daemon lifecycle restart: restart daemon against the same SQLite database
	// and NNC.
	f.RestartDaemon()

	// Rehydration verification: assert no spurious scale-up requests were
	// generated.
	f.Eventually("daemon rehydrates state and maintains stable allocations", func() (bool, string) {
		currentNNC := f.NNC()
		var unmet []string
		if !integration.StatusReady(currentNNC) {
			unmet = append(unmet, "NNC Ready condition is false")
		}
		if !apiequality.Semantic.DeepEqual(currentNNC.Spec.Allocations, allocationsBeforeRestart) {
			unmet = append(unmet, fmt.Sprintf("NNC spec allocations: got %v, want %v", currentNNC.Spec.Allocations, allocationsBeforeRestart))
		}
		return integration.FormatUnmet(unmet)
	})

	// Operational continuation: verify new pod can be allocated immediately from
	// existing capacity.
	ipNew, cidrNew := f.AllocatePod("pod-rehydrate-post-restart")
	if ipNew == "" || cidrNew == "" {
		t.Fatalf("failed to allocate pod after daemon restart")
	}
}
