package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type settingKey struct {
	source int
	id     planID
}

type mockPower struct {
	original, current, copy      planID
	values                       map[settingKey]uint32
	mutations                    []planID
	deleted                      bool
	writeFailure, restoreFailure bool
}

func fake() *mockPower {
	id := newPlanID()
	return &mockPower{
		original: id, current: id,
		values: map[settingKey]uint32{
			{0, maximum[0]}: 100, {0, minimum[0]}: 95,
			{1, maximum[0]}: 50, {1, minimum[0]}: 5,
			{0, maximum[1]}: 100, {0, minimum[1]}: 5,
			{0, boost}: 2,
		},
	}
}
func (m *mockPower) active() (planID, error) { return m.current, nil }
func (m *mockPower) activate(id planID) error {
	if m.restoreFailure && id == m.original {
		return errors.New("restore blocked")
	}
	m.current = id
	m.mutations = append(m.mutations, id)
	return nil
}
func (m *mockPower) duplicate(source, target planID) error {
	if source != m.original {
		return errors.New("wrong original plan")
	}
	m.copy = target
	return nil
}
func (m *mockPower) delete(id planID) error {
	if id == m.current || id != m.copy {
		return errors.New("wrong deletion")
	}
	m.deleted = true
	return nil
}
func (m *mockPower) read(id planID, source int, setting planID, optional bool) (uint32, bool, error) {
	if id != m.copy {
		return 0, false, errors.New("wrong plan")
	}
	value, exists := m.values[settingKey{source, setting}]
	if !exists && !optional {
		return 0, false, errors.New("missing required setting")
	}
	return value, exists, nil
}
func (m *mockPower) write(id planID, source int, setting planID, value uint32) error {
	if id != m.copy || m.writeFailure {
		return errors.New("write blocked")
	}
	m.values[settingKey{source, setting}] = value
	return nil
}
func assertRestored(t *testing.T, path string, m *mockPower) {
	t.Helper()
	if m.current != m.original || !m.deleted {
		t.Fatal("temporary power plan was not fully restored")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected journal status: %v", err)
	}
}
func TestSessionEndsAndRetainsExistingLowerLimit(t *testing.T) {
	for _, action := range []string{"expire", "stop", "window-eof", "switch-plan"} {
		t.Run(action, func(t *testing.T) {
			m := fake()
			path := filepath.Join(t.TempDir(), "restore.json")
			stop := make(chan struct{})
			read, write, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer read.Close()
			defer write.Close()
			var signal <-chan struct{} = stop
			if action == "window-eof" {
				signal = monitorWindow(read)
			}
			duration := time.Minute
			if action == "expire" {
				duration = 0
			}
			if action == "switch-plan" {
				duration = 5 * time.Second
			}
			external := newPlanID()
			result, err := session(m, path, duration, 75, signal, func(report update) {
				if report.Event != "started" || m.current != m.copy {
					t.Fatal("started before activating the temporary power plan")
				}
				want := map[settingKey]uint32{
					{0, maximum[0]}: 75, {0, minimum[0]}: 75,
					{1, maximum[0]}: 50, {1, minimum[0]}: 5,
					{0, maximum[1]}: 75, {0, boost}: 0,
				}
				for key, value := range want {
					if m.values[key] != value {
						t.Errorf("%v: got %d, want %d", key, m.values[key], value)
					}
				}
				switch action {
				case "stop":
					close(stop)
				case "window-eof":
					write.Close()
				case "switch-plan":
					m.activate(external)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if action == "switch-plan" {
				if m.current != external || !m.deleted || !strings.Contains(result, "변경한 전원 계획") {
					t.Fatal("overrode the user's selected plan")
				}
			} else {
				assertRestored(t, path, m)
			}
		})
	}
}
func TestRollbackOnWriteError(t *testing.T) {
	m := fake()
	m.writeFailure = true
	path := filepath.Join(t.TempDir(), "restore.json")
	_, err := session(m, path, 0, 75, make(chan struct{}), func(update) {
		t.Error("reported a session despite a policy error")
	})
	if err == nil || len(m.mutations) != 0 {
		t.Fatal("failed write activated the temporary plan")
	}
	assertRestored(t, path, m)
}
func TestRetryRecoveryUsingPreviousVersionRecord(t *testing.T) {
	const previous = `{"original":"381b4222-f694-41f0-9685-ff5bb260df2e","temporary":"dd0d8e58-12c9-4a9d-ab0d-a3a42a10f8e1"}`
	var record planRecord
	if err := json.Unmarshal([]byte(previous), &record); err != nil {
		t.Fatal(err)
	}
	m := fake()
	m.original, m.copy, m.current = record.Original, record.Temporary, record.Temporary
	path := filepath.Join(t.TempDir(), "restore.json")
	if err := os.WriteFile(path, []byte(previous), 0600); err != nil {
		t.Fatal(err)
	}
	m.restoreFailure = true
	if err := recoverPlan(m, path); err == nil {
		t.Fatal("restore failure was swallowed")
	}
	if m.current != m.copy || m.deleted {
		t.Fatal("failed recovery deleted the active plan")
	}
	m.restoreFailure = false
	if err := recoverPlan(m, path); err != nil {
		t.Fatal(err)
	}
	assertRestored(t, path, m)
}
func TestMalformedGUIDDoesNotPanic(t *testing.T) {
	var record planRecord
	if err := json.Unmarshal([]byte(`{"original":"------------------------------------"}`), &record); err == nil {
		t.Fatal("accepted malformed GUID")
	}
}
