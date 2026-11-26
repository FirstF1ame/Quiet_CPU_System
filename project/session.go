package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type planID [16]byte

func (id planID) String() string {
	b := id
	b[0], b[1], b[2], b[3] = id[3], id[2], id[1], id[0]
	b[4], b[5], b[6], b[7] = id[5], id[4], id[7], id[6]
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func (id planID) MarshalJSON() ([]byte, error) { return json.Marshal(id.String()) }

func (id *planID) UnmarshalJSON(raw []byte) error {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return err
	}
	return id.parse(s)
}

func (id *planID) parse(s string) error {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return errors.New("전원 계획 ID 형식이 잘못되었습니다")
	}
	b, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	if err != nil || len(b) != len(id) {
		return errors.New("전원 계획 ID 형식이 잘못되었습니다")
	}
	copy(id[:], b)
	id[0], id[1], id[2], id[3] = b[3], b[2], b[1], b[0]
	id[4], id[5], id[6], id[7] = b[5], b[4], b[7], b[6]
	return nil
}

func guid(s string) planID {
	var id planID
	if err := id.parse(s); err != nil {
		panic(err)
	}
	return id
}

func newPlanID() planID {
	var id planID
	rand.Read(id[:])
	id[7] = (id[7] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80
	return id
}

var (
	processor = guid("54533251-82be-4824-96c1-47b60b740d00")
	boost     = guid("be337238-0d82-4146-a960-4f3749d470c7")
	maximum   = []planID{
		guid("bc5038f7-23e0-4960-96da-33abaf5935ec"),
		guid("bc5038f7-23e0-4960-96da-33abaf5935ed"),
		guid("bc5038f7-23e0-4960-96da-33abaf5935ee"),
	}
	minimum = []planID{
		guid("893dee8e-2bef-41e0-89c6-b55d0929964c"),
		guid("893dee8e-2bef-41e0-89c6-b55d0929964d"),
		guid("893dee8e-2bef-41e0-89c6-b55d0929964e"),
	}
)

type power interface {
	active() (planID, error)
	activate(planID) error
	duplicate(planID, planID) error
	delete(planID) error
	read(planID, int, planID, bool) (uint32, bool, error)
	write(planID, int, planID, uint32) error
}

type planRecord struct {
	Original  planID `json:"original"`
	Temporary planID `json:"temporary"`
}

func writeRecord(path string, record planRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "restore-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	err = errors.Join(err, tmp.Close())
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func recoverPlan(p power, path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var record planRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return err
	}
	if record.Original == (planID{}) || record.Temporary == (planID{}) || record.Original == record.Temporary {
		return errors.New("복구 파일에 전원 계획을 구분할 정보가 없습니다")
	}
	current, err := p.active()
	if err != nil {
		return err
	}
	if current == record.Temporary {
		if err := p.activate(record.Original); err != nil {
			return err
		}
		current, err = p.active()
		if err != nil {
			return err
		}
		if current == record.Temporary {
			return errors.New("기존 전원 계획으로 돌아가지 못했습니다")
		}
	}
	if err := p.delete(record.Temporary); err != nil {
		return err
	}
	return os.Remove(path)
}

func configurePlan(p power, id planID, cap uint32) error {
	for source := range 2 {
		for class, setting := range maximum {
			maxValue, found, err := p.read(id, source, setting, class != 0)
			if err != nil {
				return err
			}
			if !found {
				continue
			}
			target := min(cap, maxValue)
			minValue, found, err := p.read(id, source, minimum[class], class != 0)
			if err != nil {
				return err
			}
			if found && minValue > target {
				if err := p.write(id, source, minimum[class], target); err != nil {
					return err
				}
			}
			if maxValue > target {
				if err := p.write(id, source, setting, target); err != nil {
					return err
				}
			}
		}
		value, found, err := p.read(id, source, boost, true)
		if err != nil {
			return err
		}
		if found && value != 0 {
			if err := p.write(id, source, boost, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

func stopped(stop <-chan struct{}) bool {
	select {
	case <-stop:
		return true
	default:
		return false
	}
}

type update struct {
	Event   string
	Message string
	EndsAt  time.Time
}

func session(p power, path string, duration time.Duration, cap uint32, stop <-chan struct{}, report func(update)) (message string, err error) {
	if stopped(stop) {
		return "시작을 취소했습니다.", nil
	}
	original, err := p.active()
	if err != nil {
		return "", err
	}
	temporary := newPlanID()
	if err := writeRecord(path, planRecord{original, temporary}); err != nil {
		return "", err
	}
	defer func() {
		if cleanup := recoverPlan(p, path); cleanup != nil {
			err = errors.Join(err, fmt.Errorf("전원 계획을 복구하지 못했습니다. 앱을 다시 실행해 복구를 시도하세요.\n%w", cleanup))
		}
	}()

	if err := p.duplicate(original, temporary); err != nil {
		return "", err
	}
	if err := configurePlan(p, temporary, cap); err != nil {
		return "", err
	}
	if stopped(stop) {
		return "시작을 취소했습니다.", nil
	}
	current, err := p.active()
	if err != nil {
		return "", err
	}
	if current != original {
		return "다른 전원 계획을 선택해 시작을 취소했습니다.", nil
	}
	if err := p.activate(temporary); err != nil {
		return "", err
	}
	current, err = p.active()
	if err != nil {
		return "", err
	}
	if current != temporary {
		return "", errors.New("임시 전원 계획을 활성화하지 못했습니다")
	}

	end := time.Now().Add(duration)
	report(update{Event: "started", EndsAt: end})
	timer := time.NewTimer(duration)
	defer timer.Stop()
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return "조용한 모드를 종료했습니다.", nil
		case <-timer.C:
			return "시간이 끝나 조용한 모드를 종료했습니다.", nil
		case <-tick.C:
			if !time.Now().Before(end.Round(0)) {
				return "시간이 끝나 조용한 모드를 종료했습니다.", nil
			}
			current, err := p.active()
			if err != nil {
				return "", err
			}
			if current != temporary {
				return "변경한 전원 계획을 유지하고 조용한 모드를 종료했습니다.", nil
			}
		}
	}
}

func monitorWindow(input io.Reader) <-chan struct{} {
	stop := make(chan struct{})
	go func() {
		defer close(stop)
		var b [1]byte
		_, _ = input.Read(b[:])
	}()
	return stop
}
