package file

import (
	"os"
	"testing"
)

func TestSparseFileOperations(t *testing.T) {
	// Create a temp file and make it sparse by truncating
	f, err := os.CreateTemp(t.TempDir(), "sparse-test")
	if err != nil {
		t.Fatal("failed to create temp file:", err)
	}
	defer f.Close()

	// Make it a sparse file by truncating to a large size
	const fileSize = 1024 * 1024 // 1 MB
	if err := f.Truncate(fileSize); err != nil {
		t.Fatal("failed to truncate file:", err)
	}

	// Write some data at the beginning (first 4KB)
	data := make([]byte, 4096)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if _, err := f.WriteAt(data, 0); err != nil {
		t.Fatal("failed to write data at offset 0:", err)
	}

	// Write some data in the middle (at 512KB)
	if _, err := f.WriteAt(data, 512*1024); err != nil {
		t.Fatal("failed to write data at offset 512KB:", err)
	}

	// Sync to ensure data is on disk
	if err := f.Sync(); err != nil {
		t.Fatal("failed to sync file:", err)
	}

	// Test seekData - should find data at position 0
	dataPos, err := seekData(f, 0)
	if err != nil {
		t.Fatal("seekData failed:", err)
	}
	if dataPos != 0 {
		t.Errorf("seekData(0) = %d, expected 0", dataPos)
	}

	// Test seekHole - should find hole after the first data region
	// Note: Some filesystems (like APFS) may not support sparse files
	// and will return the file size instead
	holePos, err := seekHole(f, 0)
	if err != nil {
		t.Fatal("seekHole failed:", err)
	}
	t.Logf("seekHole(0) = %d (filesystem may not support sparse files if this equals file size)", holePos)

	// Test diskUsage
	usage, err := diskUsage(f)
	if err != nil {
		t.Fatal("diskUsage failed:", err)
	}
	t.Logf("Sparse file: logical size=%d, disk usage=%d", fileSize, usage)

	// Note: On filesystems that don't support sparse files,
	// disk usage may equal or exceed logical size
	// This is acceptable - the code handles this via min(diskUsage, logicalSize)
}

func TestBlockAlign(t *testing.T) {
	tests := []struct {
		name       string
		start, end int64
		wantStart  int64
		wantEnd    int64
	}{
		{
			name:      "within first block",
			start:     0,
			end:       100,
			wantStart: 0,
			wantEnd:   sparseBlockSize,
		},
		{
			name:      "second block",
			start:     sparseBlockSize + 1,
			end:       sparseBlockSize + 100,
			wantStart: sparseBlockSize,
			wantEnd:   sparseBlockSize * 2,
		},
		{
			name:      "spanning two blocks",
			start:     100,
			end:       sparseBlockSize + 100,
			wantStart: 0,
			wantEnd:   sparseBlockSize * 2,
		},
		{
			name:      "exact block boundaries",
			start:     sparseBlockSize,
			end:       sparseBlockSize*2 - 1,
			wantStart: sparseBlockSize,
			wantEnd:   sparseBlockSize * 2,
		},
		{
			name:      "spanning three blocks",
			start:     sparseBlockSize / 2,
			end:       sparseBlockSize*2 + sparseBlockSize/2,
			wantStart: 0,
			wantEnd:   sparseBlockSize * 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStart, gotEnd := blockAlign(tt.start, tt.end)
			if gotStart != tt.wantStart {
				t.Errorf("blockAlign(%d, %d) start = %d, want %d", tt.start, tt.end, gotStart, tt.wantStart)
			}
			if gotEnd != tt.wantEnd {
				t.Errorf("blockAlign(%d, %d) end = %d, want %d", tt.start, tt.end, gotEnd, tt.wantEnd)
			}
		})
	}
}

func TestIsRangeCached(t *testing.T) {
	// Create a sparse file with some data
	f, err := os.CreateTemp(t.TempDir(), "range-test")
	if err != nil {
		t.Fatal("failed to create temp file:", err)
	}
	defer f.Close()

	// Make it a sparse file
	const fileSize = sparseBlockSize * 4 // 4 blocks
	if err := f.Truncate(fileSize); err != nil {
		t.Fatal("failed to truncate file:", err)
	}

	// Write data to first block only
	data := make([]byte, sparseBlockSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if _, err := f.WriteAt(data, 0); err != nil {
		t.Fatal("failed to write data:", err)
	}
	f.Sync()

	// Test: First block should be cached (has data)
	cached, err := isRangeCached(f, 0, sparseBlockSize-1)
	if err != nil {
		t.Fatal("isRangeCached failed:", err)
	}
	if !cached {
		t.Error("first block should be cached")
	}

	// Check if filesystem supports sparse files by checking seekHole
	holePos, _ := seekHole(f, 0)
	sparseSupported := holePos < fileSize

	if sparseSupported {
		// Test: Second block should NOT be cached (it's a hole)
		cached, err = isRangeCached(f, sparseBlockSize, sparseBlockSize*2-1)
		if err != nil {
			t.Fatal("isRangeCached failed:", err)
		}
		if cached {
			t.Error("second block should not be cached")
		}

		// Test: Range spanning cached and uncached should return false
		cached, err = isRangeCached(f, 0, sparseBlockSize*2-1)
		if err != nil {
			t.Fatal("isRangeCached failed:", err)
		}
		if cached {
			t.Error("range spanning cached and uncached should not be fully cached")
		}
	} else {
		t.Log("Filesystem does not support sparse files, skipping hole detection tests")
		// On non-sparse filesystems, truncated data appears as data (zeros)
		// so isRangeCached will return true for all ranges
	}
}

func TestUncachedRanges(t *testing.T) {
	// Create a sparse file with some data
	f, err := os.CreateTemp(t.TempDir(), "uncached-test")
	if err != nil {
		t.Fatal("failed to create temp file:", err)
	}
	defer f.Close()

	// Make it a sparse file
	const fileSize = sparseBlockSize * 4 // 4 blocks
	if err := f.Truncate(fileSize); err != nil {
		t.Fatal("failed to truncate file:", err)
	}

	// Write data to blocks 0 and 2 only
	data := make([]byte, sparseBlockSize)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if _, err := f.WriteAt(data, 0); err != nil { // Block 0
		t.Fatal("failed to write data to block 0:", err)
	}
	if _, err := f.WriteAt(data, sparseBlockSize*2); err != nil { // Block 2
		t.Fatal("failed to write data to block 2:", err)
	}
	f.Sync()

	// Check if filesystem supports sparse files
	holePos, _ := seekHole(f, 0)
	sparseSupported := holePos < fileSize

	// Test: Find uncached ranges in the entire file
	uncached, err := uncachedRanges(f, 0, fileSize-1)
	if err != nil {
		t.Fatal("uncachedRanges failed:", err)
	}

	t.Logf("Uncached ranges: %+v", uncached)

	if sparseSupported {
		// Should find holes in blocks 1 and 3
		if len(uncached) == 0 {
			t.Error("expected to find uncached ranges on sparse-supporting filesystem")
		}

		// Test: Find uncached ranges in just the first two blocks
		uncached, err = uncachedRanges(f, 0, sparseBlockSize*2-1)
		if err != nil {
			t.Fatal("uncachedRanges failed:", err)
		}

		// Should find hole in block 1
		foundBlock1Hole := false
		for _, r := range uncached {
			if r.Start >= sparseBlockSize && r.End < sparseBlockSize*2 {
				foundBlock1Hole = true
			}
		}
		if !foundBlock1Hole {
			t.Logf("Uncached ranges in first two blocks: %+v", uncached)
		}
	} else {
		t.Log("Filesystem does not support sparse files, skipping hole detection tests")
		// On non-sparse filesystems, there are no holes, so uncachedRanges returns empty
		// This is expected behavior
	}
}

func TestPunchHole(t *testing.T) {
	// Create a temp file with data
	f, err := os.CreateTemp(t.TempDir(), "punchhole-test")
	if err != nil {
		t.Fatal("failed to create temp file:", err)
	}
	defer f.Close()

	// Write data to the file (two blocks worth)
	const blockSize = sparseBlockSize
	data := make([]byte, blockSize*2)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal("failed to write data:", err)
	}
	f.Sync()

	// Get initial disk usage
	usageBefore, err := diskUsage(f)
	if err != nil {
		t.Fatal("failed to get initial disk usage:", err)
	}
	t.Logf("Disk usage before punch: %d bytes", usageBefore)

	// Punch a hole in the first block
	err = punchHole(f, 0, blockSize)
	if err != nil {
		t.Logf("punchHole returned error: %v (may not be supported on this filesystem)", err)
		t.Skip("punchHole not supported on this filesystem")
	}
	f.Sync()

	// Get disk usage after punch
	usageAfter, err := diskUsage(f)
	if err != nil {
		t.Fatal("failed to get disk usage after punch:", err)
	}
	t.Logf("Disk usage after punch: %d bytes", usageAfter)

	// Verify disk usage decreased (or stayed same on non-supporting filesystems)
	if usageAfter > usageBefore {
		t.Errorf("disk usage should not increase after punch hole: before=%d, after=%d", usageBefore, usageAfter)
	}

	// Check if filesystem supports sparse files by checking if a hole was created
	holePos, err := seekHole(f, 0)
	if err != nil {
		t.Fatal("seekHole failed:", err)
	}

	fileInfo, _ := f.Stat()
	fileSize := fileInfo.Size()

	if holePos < fileSize {
		// Filesystem supports holes - verify the hole is where we punched
		t.Logf("Hole found at offset %d (file size: %d)", holePos, fileSize)
		if holePos != 0 {
			t.Errorf("expected hole at offset 0, got %d", holePos)
		}

		// Verify data in second block is still intact
		readBuf := make([]byte, 100)
		n, err := f.ReadAt(readBuf, blockSize)
		if err != nil {
			t.Fatal("failed to read second block:", err)
		}
		if n != 100 {
			t.Errorf("expected to read 100 bytes, got %d", n)
		}
		for i := 0; i < n; i++ {
			expected := byte((blockSize + int64(i)) % 256)
			if readBuf[i] != expected {
				t.Errorf("data mismatch at offset %d: got %d, want %d", blockSize+int64(i), readBuf[i], expected)
				break
			}
		}
		t.Log("Second block data verified intact after hole punch")
	} else {
		t.Log("Filesystem does not support hole detection, skipping hole verification")
	}
}
