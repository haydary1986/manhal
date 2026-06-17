package predator

import "testing"

func TestListEdit(t *testing.T) {
	l := NewList(nil)

	if err := l.Add("foo journal", "ناشر مشبوه"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if l.Len() != 1 {
		t.Fatalf("len = %d, want 1", l.Len())
	}

	// Case-insensitive duplicate is rejected.
	if err := l.Add("FOO Journal", "dup"); err != ErrDuplicate {
		t.Errorf("duplicate => %v, want ErrDuplicate", err)
	}
	if err := l.Add("", "no pattern"); err == nil {
		t.Error("empty pattern should error")
	}

	// The added pattern matches in Check.
	if m := l.Check("The Foo Journal of Things"); len(m) != 1 {
		t.Errorf("Check should match added pattern, got %v", m)
	}

	if err := l.Remove("foo journal"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if l.Len() != 0 {
		t.Errorf("len after remove = %d, want 0", l.Len())
	}
	if err := l.Remove("nope"); err != ErrNotFound {
		t.Errorf("missing => %v, want ErrNotFound", err)
	}
}
