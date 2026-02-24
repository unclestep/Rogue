package idgen

import (
	"testing"
)

func TestIDGenerationSequence(t *testing.T) {
	// Check sequentiality
	id1 := Next()
	id2 := Next()
	id3 := Next()

	if id1 == id2 || id2 == id3 {
		t.Error("GenID produced duplicate values")
	}
	if id2 != id1+1 || id3 != id2+1 {
		t.Error("GenID is not incremental")
	}

	SetStartID(0)
	id4 := Next()

	if id4 != 1 {
		t.Errorf("GenID should be reseted to 0\nexpected: 1, got: %v", id4)
	}

	id5 := Current()

	if id5 == id4 {
		t.Errorf("Should be identical. id4=%v id5=%v", id4, id5)
	}
}
