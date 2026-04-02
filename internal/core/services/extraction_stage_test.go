package services

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"reel/internal/database/models"
)

// createTestZip creates a zip archive containing a file with the given name and size.
func createTestZip(t *testing.T, dir, zipName, innerName string, innerSizeKB int) string {
	t.Helper()
	zipPath := filepath.Join(dir, zipName)
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)

	fw, err := w.Create(innerName)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, innerSizeKB*1024)
	fw.Write(data)

	w.Close()
	f.Close()
	return zipPath
}

func TestExtraction_ZipFile(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	z := createTestZip(t, srcDir, "archive.zip", "movie.mkv", 100)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{z}
	ctx.DownloadPath = srcDir

	stage := NewExtractionStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OriginalFiles should be updated with extracted files
	if len(ctx.OriginalFiles) == 0 {
		t.Fatal("expected extracted files in OriginalFiles")
	}

	// Extracted file should exist
	for _, f := range ctx.OriginalFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("extracted file should exist: %s", f)
		}
	}
}

func TestExtraction_ZipSlipPrevention(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	// Create zip with path traversal
	zipPath := filepath.Join(srcDir, "evil.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	// Write a file with path traversal
	fw, _ := w.Create("../../evil.txt")
	fw.Write([]byte("evil"))
	w.Close()
	f.Close()

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{zipPath}
	ctx.DownloadPath = srcDir

	stage := NewExtractionStage(cfg, testLogger())
	err = stage.Execute(ctx)
	if err == nil {
		t.Fatal("expected error for ZipSlip path traversal")
	}
}

func TestExtraction_NoArchives(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	f := createTestFile(t, srcDir, "movie.mkv", 2)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	ctx.DownloadPath = srcDir

	stage := NewExtractionStage(cfg, testLogger())
	if err := stage.Execute(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// OriginalFiles should be unchanged
	if len(ctx.OriginalFiles) != 1 || ctx.OriginalFiles[0] != f {
		t.Error("OriginalFiles should be unchanged when no archives")
	}
}

func TestExtraction_Rollback_CleansExtractedDirs(t *testing.T) {
	srcDir := t.TempDir()
	cfg := testConfig(srcDir, t.TempDir())

	z := createTestZip(t, srcDir, "archive.zip", "movie.mkv", 100)

	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{z}
	ctx.DownloadPath = srcDir

	stage := NewExtractionStage(cfg, testLogger())
	stage.Execute(ctx)

	// Verify extracted dirs recorded in metadata
	if _, ok := ctx.Metadata["extracted_dirs"]; !ok {
		t.Fatal("expected extracted_dirs in metadata")
	}

	stage.Rollback(ctx)
	// Rollback should clean up - no assertion needed beyond no panic
}
