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
	"testing"

	nncv1 "github.com/GoogleCloudPlatform/gke-networking-api/apis/nodenetworkconfig/v1"
	"k8s.io/cloud-provider-gcp/pkg/controller/dynamicpodip"
	"k8s.io/metis/pkg/store"
	"k8s.io/metis/test/integration"
)

// TestScaleUp_ANE exercises dynamic scale-up using the Alias Network Endpoint
// (ANE) backend. The daemon does not know about ANEs specifically, but receives
// them as /32 CIDR allocations with a NonReusable policy.
func TestScaleUp_ANE(t *testing.T) {
	f := integration.NewFixture(t, integration.WithBackend(dynamicpodip.BackendTypeANE))

	// Bootstrap: allocate pods to exhaust the CNI-provided initial block.
	for i := 1; i <= integration.InitialUsableIPs; i++ {
		f.AllocatePod(fmt.Sprintf("pod-ane-init-%d", i))
	}

	// ANE scale-up demand: an overflow pod triggers the daemon to request
	// additional capacity.
	_, dynamicCIDR := f.AllocatePod("pod-ane-overflow")
	if dynamicCIDR == integration.InitialPodCIDR {
		t.Fatalf("overflow pod allocated from initial range; expected an ANE /32 allocation")
	}

	// Verification of ANE allocation: GCE creates /32 ANE, status publishes /32
	// with NonReusable policy.
	f.Eventually("GCE creates ANE and status publishes /32 with NonReusable policy", func(a *integration.Assert) {
		currentNNC := f.NNC()
		hasStatus32 := false
		for _, pc := range currentNNC.Status.PodCIDRs {
			if pc.CIDR == dynamicCIDR && pc.ReusePolicy == nncv1.PodCIDRTypeNonReusable {
				hasStatus32 = true
				break
			}
		}
		a.True("GCE has ANEs", len(f.GCEANEs()) > 0)
		a.True("NNC status has NonReusable /32 CIDR", hasStatus32)
		a.True("NNC Ready condition", integration.StatusReady(currentNNC))
		a.Equal("DB block state", f.BlockState(dynamicCIDR), store.StateReady)
	})
}
