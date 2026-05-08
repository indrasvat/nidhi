package layout

import (
	"testing"
)

func TestComputeSplit_Preview(t *testing.T) {
	// PREVIEW mode: 40/60 vertical split on a 40-line content area.
	result := ComputeSplit(40, PreviewSplitRatio)

	if result.PrimarySize < PreviewSplitRatio.MinPrimary {
		t.Errorf("PrimarySize = %d, want >= %d (MinPrimary)", result.PrimarySize, PreviewSplitRatio.MinPrimary)
	}
	if result.SecondarySize < PreviewSplitRatio.MinSecondary {
		t.Errorf("SecondarySize = %d, want >= %d (MinSecondary)", result.SecondarySize, PreviewSplitRatio.MinSecondary)
	}
	if result.DividerSize != 1 {
		t.Errorf("DividerSize = %d, want 1", result.DividerSize)
	}

	total := result.PrimarySize + result.SecondarySize + result.DividerSize
	if total != 40 {
		t.Errorf("total %d + %d + %d = %d, want 40",
			result.PrimarySize, result.SecondarySize, result.DividerSize, total)
	}
}

func TestComputeSplit_Detail(t *testing.T) {
	// DETAIL mode: 25/75 horizontal split on 120-col content area.
	result := ComputeSplit(120, DetailSplitRatio)

	if result.PrimarySize < DetailSplitRatio.MinPrimary {
		t.Errorf("PrimarySize = %d, want >= %d (MinPrimary)", result.PrimarySize, DetailSplitRatio.MinPrimary)
	}
	if result.SecondarySize < DetailSplitRatio.MinSecondary {
		t.Errorf("SecondarySize = %d, want >= %d (MinSecondary)", result.SecondarySize, DetailSplitRatio.MinSecondary)
	}

	total := result.PrimarySize + result.SecondarySize + result.DividerSize
	if total != 120 {
		t.Errorf("total = %d, want 120", total)
	}
}

func TestComputeSplit_TooSmall(t *testing.T) {
	// Not enough space for both panes.
	result := ComputeSplit(10, DetailSplitRatio)

	if result.SecondarySize != 0 {
		t.Errorf("SecondarySize = %d, want 0 (collapsed)", result.SecondarySize)
	}
	if result.DividerSize != 0 {
		t.Errorf("DividerSize = %d, want 0 (collapsed)", result.DividerSize)
	}
}

func TestComputeSplit_MinimumEnforced(t *testing.T) {
	// Just barely enough for both panes.
	minTotal := PreviewSplitRatio.MinPrimary + PreviewSplitRatio.MinSecondary + 1
	result := ComputeSplit(minTotal, PreviewSplitRatio)

	if result.PrimarySize < PreviewSplitRatio.MinPrimary {
		t.Errorf("PrimarySize = %d, below minimum %d",
			result.PrimarySize, PreviewSplitRatio.MinPrimary)
	}
	if result.SecondarySize < PreviewSplitRatio.MinSecondary {
		t.Errorf("SecondarySize = %d, below minimum %d",
			result.SecondarySize, PreviewSplitRatio.MinSecondary)
	}
}

func TestComputeSplit_SumsCorrectly(t *testing.T) {
	for totalSize := 1; totalSize <= 200; totalSize++ {
		result := ComputeSplit(totalSize, PreviewSplitRatio)
		actual := result.PrimarySize + result.SecondarySize + result.DividerSize
		if result.SecondarySize > 0 && actual != totalSize {
			t.Errorf("totalSize=%d: %d + %d + %d = %d, want %d",
				totalSize, result.PrimarySize, result.SecondarySize, result.DividerSize,
				actual, totalSize)
		}
	}
}

func TestComputeSplit_CollapsedTotalDoesNotExceedInput(t *testing.T) {
	for totalSize := 0; totalSize <= 20; totalSize++ {
		result := ComputeSplit(totalSize, DetailSplitRatio)
		if result.SecondarySize == 0 {
			if result.PrimarySize > totalSize {
				t.Errorf("totalSize=%d: collapsed PrimarySize=%d exceeds total",
					totalSize, result.PrimarySize)
			}
		}
	}
}
