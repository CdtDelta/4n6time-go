package importer

import "testing"

// TestImportFileResultCarriesUnparseable checks that a single-file import
// returns the parser's unparseable count on its result, which ImportCSV passes
// to the frontend. A clean file returns zero.
func TestImportFileResultCarriesUnparseable(t *testing.T) {
	dir := t.TempDir()
	biome := writeTmpCSV(t, dir, "Biome.csv", maBiomeHeader+"\n"+maBiomeBadTS)
	clean := writeTmpCSV(t, dir, "FsEvents.csv", maFsCSV())
	logf := func(level, msg string) {}

	format, label, err := DetectFile(biome)
	if err != nil {
		t.Fatalf("DetectFile(biome): %v", err)
	}
	res, err := ImportFile(biome, format, label, "", &mockStore{}, &Progress{}, logf)
	if err != nil {
		t.Fatalf("ImportFile(biome): %v", err)
	}
	if res.Unparseable != 1 {
		t.Errorf("biome: TrackedResult.Unparseable = %d, want 1", res.Unparseable)
	}

	format, label, err = DetectFile(clean)
	if err != nil {
		t.Fatalf("DetectFile(clean): %v", err)
	}
	res, err = ImportFile(clean, format, label, "", &mockStore{}, &Progress{}, logf)
	if err != nil {
		t.Fatalf("ImportFile(clean): %v", err)
	}
	if res.Unparseable != 0 {
		t.Errorf("clean: TrackedResult.Unparseable = %d, want 0", res.Unparseable)
	}
}
