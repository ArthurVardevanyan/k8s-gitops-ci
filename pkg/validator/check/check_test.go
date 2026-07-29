package check

import "testing"

type dummyCheck struct{ id string }

func (d dummyCheck) ID() string      { return d.id }
func (d dummyCheck) Title() string   { return d.id }
func (d dummyCheck) Section() string { return "sec" }
func (d dummyCheck) Blocking() bool  { return false }
func (d dummyCheck) Scope() Scope    { return ScopeDoc }

func TestRegister_PanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on empty id")
		}
	}()
	Register(dummyCheck{id: ""})
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	Register(dummyCheck{id: "dup-test"})
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on duplicate id")
		}
	}()
	Register(dummyCheck{id: "dup-test"})
}

func TestByID(t *testing.T) {
	Register(dummyCheck{id: "byid-test"})
	c, ok := ByID("byid-test")
	if !ok || c.ID() != "byid-test" {
		t.Error("expected check by id")
	}
}

func TestByScope(t *testing.T) {
	Register(dummyCheck{id: "scope-test"})
	out := ByScope(ScopeDoc)
	found := false
	for _, c := range out {
		if c.ID() == "scope-test" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected scope check in list: %v", out)
	}
}

func TestFindingScalar(t *testing.T) {
	f := Finding{Value: "x", Path: "/spec", File: "a.yaml", Kind: "Deployment", Name: "d"}
	s := f.Scalar()
	if s.Value != "x" || s.Kind != "Deployment" {
		t.Errorf("unexpected scalar: %+v", s)
	}
}
