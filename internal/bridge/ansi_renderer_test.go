package bridge
package bridge

import (
	"reflect"
	"testing"
)

func TestRenderBatchToLinesWithState_PreservesScreenAcrossBatches(t *testing.T) {
	screen := newANSIScreen()

	first := renderBatchToLinesWithState("first\nsecond", screen)
	if !reflect.DeepEqual(first, []string{"first", "second"}) {
		t.Fatalf("unexpected first frame: %#v", first)
	}

	second := renderBatchToLinesWithState("\x1b[1;1Hupdated", screen)
	want := []string{"updated", "second"}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("unexpected second frame: got %#v want %#v", second, want)
	}
}

func TestRenderBatchToLines_IsolatedPerBatch(t *testing.T) {
	first := renderBatchToLines("first\nsecond")
	if !reflect.DeepEqual(first, []string{"first", "second"}) {
		t.Fatalf("unexpected first frame: %#v", first)
	}

	second := renderBatchToLines("\x1b[1;1Hupdated")
	want := []string{"updated"}
	if !reflect.DeepEqual(second, want) {
		t.Fatalf("unexpected second frame: got %#v want %#v", second, want)
	}
}
