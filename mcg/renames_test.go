// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcg_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	txtpbfmtast "github.com/protocolbuffers/txtpbfmt/ast"
	txtpbfmt "github.com/protocolbuffers/txtpbfmt/parser"
	"sdv.googlesource.com/mcg/mcg"
)

// stripComments removes all comments from the AST nodes recursively.
func stripComments(nodes []*txtpbfmtast.Node) {
	for _, n := range nodes {
		n.PreComments = nil
		if n.Children != nil {
			stripComments(n.Children)
		}
	}
}

func TestApplyLegacyToCanonicalRenames(t *testing.T) {
	inputBytes := FileAsBytes(t, "testdata/v1_legacy_fixture.textproto")

	nodes, err := txtpbfmt.Parse(inputBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse failed: %v", err)
	}

	mcg.ApplyLegacyToCanonicalRenames(nodes)

	stripComments(nodes)

	got := string(txtpbfmt.PrettyBytes(nodes, 0))
	wantBytes := FileAsBytes(t, "testdata/v2_canonical_fixture.textproto")
	wantNodes, err := txtpbfmt.Parse(wantBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse(want) failed: %v", err)
	}
	want := string(txtpbfmt.PrettyBytes(wantNodes, 0))

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ApplyLegacyToCanonicalRenames() mismatch (-want +got):\n%s", diff)
	}
}

func TestApplyCanonicalToLegacyRenames(t *testing.T) {
	inputBytes := FileAsBytes(t, "testdata/v2_canonical_fixture.textproto")

	nodes, err := txtpbfmt.Parse(inputBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse failed: %v", err)
	}

	mcg.ApplyCanonicalToLegacyRenames(nodes)

	wantBytes := FileAsBytes(t, "testdata/v1_legacy_fixture.textproto")
	wantNodes, err := txtpbfmt.Parse(wantBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse(want) failed: %v", err)
	}
	stripComments(wantNodes)

	got := string(txtpbfmt.PrettyBytes(nodes, 0))
	want := string(txtpbfmt.PrettyBytes(wantNodes, 0))

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ApplyCanonicalToLegacyRenames() mismatch (-want +got):\n%s", diff)
	}
}

func TestRenamesRoundTrip(t *testing.T) {
	// Start with Canonical
	inputBytes := FileAsBytes(t, "testdata/v2_canonical_fixture.textproto")
	nodes, err := txtpbfmt.Parse(inputBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse failed: %v", err)
	}

	// Canonical -> Legacy
	mcg.ApplyCanonicalToLegacyRenames(nodes)

	// Legacy -> Canonical
	mcg.ApplyLegacyToCanonicalRenames(nodes)

	// Should be back to original (Canonical)
	got := string(txtpbfmt.PrettyBytes(nodes, 0))

	wantNodes, err := txtpbfmt.Parse(inputBytes)
	if err != nil {
		t.Fatalf("txtpbfmt.Parse(want) failed: %v", err)
	}
	want := string(txtpbfmt.PrettyBytes(wantNodes, 0))

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Round Trip mismatch (-want +got):\n%s", diff)
	}
}
