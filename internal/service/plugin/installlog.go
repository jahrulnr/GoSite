package plugin

import (
	"encoding/json"
	"time"
)

type installLogStep struct {
	Step         string `json:"step"`
	At           string `json:"at"`
	Status       string `json:"status"`
	FailureClass string `json:"failure_class,omitempty"`
	Detail       string `json:"detail,omitempty"`
}

type installLog struct {
	steps []installLogStep
}

func newInstallLog() *installLog {
	return &installLog{}
}

func (l *installLog) ok(step string) {
	l.steps = append(l.steps, installLogStep{
		Step:   step,
		At:     time.Now().UTC().Format(time.RFC3339),
		Status: "ok",
	})
}

func (l *installLog) fail(step, failureClass, detail string) {
	l.steps = append(l.steps, installLogStep{
		Step:         step,
		At:           time.Now().UTC().Format(time.RFC3339),
		Status:       "failed",
		FailureClass: failureClass,
		Detail:       detail,
	})
}

func (l *installLog) JSON() string {
	if len(l.steps) == 0 {
		return "[]"
	}
	b, err := json.Marshal(l.steps)
	if err != nil {
		return "[]"
	}
	return string(b)
}
