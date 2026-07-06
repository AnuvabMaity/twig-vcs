package objects

import (
	"reflect"
	"testing"
)

func TestSortTreeEntries(t *testing.T) {
	// A shuffled slice of 5 entries
	entries := []TreeEntry{
		{Name: "delta.txt", Hash: "h4", Type: TypeBlob},
		{Name: "alpha.txt", Hash: "h1", Type: TypeBlob},
		{Name: "echo.txt", Hash: "h5", Type: TypeBlob},
		{Name: "beta.txt", Hash: "h2", Type: TypeBlob},
		{Name: "gamma.txt", Hash: "h3", Type: TypeBlob},
	}

	expected := []TreeEntry{
		{Name: "alpha.txt", Hash: "h1", Type: TypeBlob},
		{Name: "beta.txt", Hash: "h2", Type: TypeBlob},
		{Name: "delta.txt", Hash: "h4", Type: TypeBlob},
		{Name: "echo.txt", Hash: "h5", Type: TypeBlob},
		{Name: "gamma.txt", Hash: "h3", Type: TypeBlob},
	}

	SortTreeEntries(entries)

	if !reflect.DeepEqual(entries, expected) {
		t.Errorf("SortTreeEntries failed. Expected: %v, got: %v", expected, entries)
	}
}
