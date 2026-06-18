package splunklite

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ApplyPipes runs post-search pipe commands on events.
func ApplyPipes(events []QueryEvent, pipes []PipeCmd) []QueryEvent {
	out := events
	for _, pipe := range pipes {
		switch pipe.Name {
		case "head":
			if pipe.Limit > 0 && len(out) > pipe.Limit {
				out = out[:pipe.Limit]
			}
		case "sort":
			out = sortEvents(out, pipe.Field, pipe.Desc)
		}
	}
	return out
}

func sortEvents(events []QueryEvent, field string, desc bool) []QueryEvent {
	if len(events) < 2 {
		return events
	}
	field = strings.ToLower(strings.TrimSpace(field))
	out := append([]QueryEvent(nil), events...)
	sort.SliceStable(out, func(i, j int) bool {
		less := compareEventField(out[i], out[j], field)
		if desc {
			return !less
		}
		return less
	})
	return out
}

func compareEventField(a, b QueryEvent, field string) bool {
	switch field {
	case "ts", "time":
		return a.TS.Before(b.TS)
	case "source":
		return a.Source < b.Source
	case "action":
		return a.Action < b.Action
	case "user":
		return a.User < b.User
	case "status", "status_code":
		return eventIntMeta(a, "status_code") < eventIntMeta(b, "status_code")
	case "message":
		return a.Message < b.Message
	default:
		return a.TS.Before(b.TS)
	}
}

func eventIntMeta(ev QueryEvent, key string) int {
	if ev.Meta == nil {
		return 0
	}
	v, ok := ev.Meta[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(n)
		return i
	default:
		return 0
	}
}

// SortEventsByTimeDesc sorts events newest first (default query merge order).
func SortEventsByTimeDesc(events []QueryEvent) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].TS.After(events[j].TS)
	})
}

// EventTS returns event timestamp for tests.
func EventTS(ev QueryEvent) time.Time { return ev.TS }

// FormatSortField validates a sort field name.
func FormatSortField(field string) (string, error) {
	field = strings.ToLower(strings.TrimSpace(field))
	switch field {
	case "ts", "time", "source", "action", "user", "status", "status_code", "message":
		return field, nil
	default:
		return "", fmt.Errorf("unsupported sort field %q", field)
	}
}
