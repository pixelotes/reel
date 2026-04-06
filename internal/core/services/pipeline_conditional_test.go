package services

import (
	"testing"

	"reel/internal/database/models"
)

func TestConditional_Always(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "always", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	stage.Execute(ctx)

	if !called {
		t.Error("always condition should execute inner stage")
	}
}

func TestConditional_Never(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "never", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	stage.Execute(ctx)

	if called {
		t.Error("never condition should not execute inner stage")
	}
}

func TestConditional_IsMovie_True(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "is_movie", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	stage.Execute(ctx)

	if !called {
		t.Error("is_movie should match movie media type")
	}
}

func TestConditional_IsMovie_False(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "is_movie", testLogger())
	ctx := testContext(t, models.MediaTypeTVShow)
	stage.Execute(ctx)

	if called {
		t.Error("is_movie should not match tvshow media type")
	}
}

func TestConditional_IsTVShow(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "is_tv", testLogger())
	ctx := testContext(t, models.MediaTypeTVShow)
	stage.Execute(ctx)

	if !called {
		t.Error("is_tv should match tvshow media type")
	}
}

func TestConditional_IsAnime(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "is_anime", testLogger())
	ctx := testContext(t, models.MediaTypeAnime)
	stage.Execute(ctx)

	if !called {
		t.Error("is_anime should match anime media type")
	}
}

func TestConditional_FileCountGreater(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "files > 2", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{"a.mkv", "b.mkv", "c.mkv"}
	stage.Execute(ctx)

	if !called {
		t.Error("files > 2 with 3 files should execute")
	}
}

func TestConditional_FileCountLess(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "files > 5", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{"a.mkv", "b.mkv"}
	stage.Execute(ctx)

	if called {
		t.Error("files > 5 with 2 files should not execute")
	}
}

func TestConditional_FileSizeGreater(t *testing.T) {
	srcDir := t.TempDir()

	// Create a 2MB file
	f := createTestFile(t, srcDir, "big.mkv", 2)

	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "file_size > 1MB", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	stage.Execute(ctx)

	if !called {
		t.Error("file_size > 1MB with 2MB file should execute")
	}
}

func TestConditional_FileSizeLess(t *testing.T) {
	srcDir := t.TempDir()

	f := createSmallFile(t, srcDir, "small.mkv", 500) // 500KB

	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "file_size > 1MB", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	ctx.OriginalFiles = []string{f}
	stage.Execute(ctx)

	if called {
		t.Error("file_size > 1MB with 500KB file should not execute")
	}
}

func TestConditional_IsEbook(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "is_ebook", testLogger())
	ctx := testContext(t, models.MediaTypeEbook)
	stage.Execute(ctx)

	if !called {
		t.Error("is_ebook should match ebook media type")
	}
}

func TestConditional_UnknownCondition_DefaultsTrue(t *testing.T) {
	called := false
	inner := &spyStage{name: "inner", executeFunc: func(ctx *ProcessingContext) error {
		called = true
		return nil
	}}

	stage := NewConditionalStage(inner, "foobar_unknown", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)
	stage.Execute(ctx)

	if !called {
		t.Error("unknown condition should default to true")
	}
}

func TestConditional_Name_ReturnsInnerName(t *testing.T) {
	inner := newSpyStage("validate")
	stage := NewConditionalStage(inner, "always", testLogger())

	if stage.Name() != "validate" {
		t.Errorf("expected name 'validate', got %q", stage.Name())
	}
}

func TestConditional_Rollback_DelegatesToInner(t *testing.T) {
	inner := newSpyStage("inner")
	stage := NewConditionalStage(inner, "always", testLogger())
	ctx := testContext(t, models.MediaTypeMovie)

	stage.Rollback(ctx)

	if !inner.rollbackCalled {
		t.Error("rollback should delegate to inner stage")
	}
}

