package instance

import "testing"

func TestLockExcludesAndReleases(t *testing.T) {
	dir := t.TempDir()
	release, err := Acquire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := Acquire(dir); err == nil {
		_ = second()
		_ = release()
		t.Fatal("second instance acquired the same lock")
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	release, err = Acquire(dir)
	if err != nil {
		t.Fatal("lock was not released:", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
}
