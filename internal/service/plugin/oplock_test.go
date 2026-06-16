package plugin

import "testing"

func TestOpLock_SerializesPlugin(t *testing.T) {
	lock := NewOpLock()
	if !lock.TryAcquire("acme/foo") {
		t.Fatal("first acquire should succeed")
	}
	if lock.TryAcquire("acme/foo") {
		t.Fatal("second acquire should fail")
	}
	if !lock.TryAcquire("other/bar") {
		t.Fatal("different plugin should succeed")
	}
	lock.Release("acme/foo")
	if !lock.TryAcquire("acme/foo") {
		t.Fatal("acquire after release should succeed")
	}
}
