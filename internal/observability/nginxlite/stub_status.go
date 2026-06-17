package nginxlite

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var stubActiveRE = regexp.MustCompile(`Active connections:\s*(\d+)`)
var stubStateRE = regexp.MustCompile(`Reading:\s*(\d+)\s+Writing:\s*(\d+)\s+Waiting:\s*(\d+)`)

// StubStatus is a parsed nginx stub_status response.
type StubStatus struct {
	Active   int
	Accepts  int64
	Handled  int64
	Requests int64
	Reading  int
	Writing  int
	Waiting  int
}

// ParseStubStatus parses the plain-text stub_status body.
func ParseStubStatus(body string) (StubStatus, error) {
	var out StubStatus
	scanner := bufio.NewScanner(strings.NewReader(body))
	var counterLine string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if m := stubActiveRE.FindStringSubmatch(line); len(m) == 2 {
			out.Active = atoi(m[1])
			continue
		}
		if strings.Contains(line, "server accepts handled requests") {
			if scanner.Scan() {
				counterLine = strings.TrimSpace(scanner.Text())
			}
			continue
		}
		if m := stubStateRE.FindStringSubmatch(line); len(m) == 4 {
			out.Reading = atoi(m[1])
			out.Writing = atoi(m[2])
			out.Waiting = atoi(m[3])
		}
	}
	if err := scanner.Err(); err != nil {
		return StubStatus{}, fmt.Errorf("scan stub_status: %w", err)
	}
	if counterLine != "" {
		fields := strings.Fields(counterLine)
		if len(fields) < 3 {
			return StubStatus{}, fmt.Errorf("parse stub_status counters: %q", counterLine)
		}
		out.Accepts = atoi64(fields[0])
		out.Handled = atoi64(fields[1])
		out.Requests = atoi64(fields[2])
	}
	if out.Active == 0 && counterLine == "" && !strings.Contains(body, "Active connections") {
		return StubStatus{}, fmt.Errorf("unrecognized stub_status body")
	}
	return out, nil
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func atoi64(s string) int64 {
	n, _ := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	return n
}
