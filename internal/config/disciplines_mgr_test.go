package config

import "testing"

func TestDisciplinesManager(t *testing.T) {
	m := NewDisciplinesManager(t.TempDir(), DefaultDisciplines())
	n := len(m.List())

	if err := m.Add("xtest", "اختبار"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if len(m.List()) != n+1 {
		t.Errorf("after add len = %d, want %d", len(m.List()), n+1)
	}
	if err := m.Add("xtest", "dup"); err != ErrDisciplineDuplicate {
		t.Errorf("duplicate => %v, want ErrDisciplineDuplicate", err)
	}
	if err := m.Add("", "no id"); err == nil {
		t.Error("empty id should error")
	}

	if err := m.Remove("xtest"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if len(m.List()) != n {
		t.Errorf("after remove len = %d, want %d", len(m.List()), n)
	}
	if err := m.Remove("missing"); err != ErrDisciplineNotFound {
		t.Errorf("missing => %v, want ErrDisciplineNotFound", err)
	}

	// nil-safe.
	var nilMgr *DisciplinesManager
	if nilMgr.List() != nil {
		t.Error("nil manager List should be nil")
	}
}
