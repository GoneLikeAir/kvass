package shard

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRuntimeInfo_FailOpenJSONFields(t *testing.T) {
	now := time.Date(2026, 9, 11, 3, 4, 5, 0, time.UTC)
	info := RuntimeInfo{
		DropSetFailOpen: 2,
		DropSetFailOpenByReason: map[string]uint64{
			"parse_error":         1,
			"unsupported_format":  1,
			"unexpected_response": 0,
			"internal_panic":      0,
		},
		DropSetLoadError: "load failed",
		DropSetLastError: "fail-open: parse_error",
		DropSetLastFailure: &DropSetLastFailure{
			Reason:          "parse_error",
			Time:            now,
			Job:             "job1",
			TargetID:        "1",
			Address:         "http://127.0.0.1:9090",
			ContentType:     "text/plain",
			FilteringActive: true,
			Generation:      "g1",
		},
	}
	b, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var got RuntimeInfo
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.DropSetFailOpen != 2 || got.DropSetFailOpenByReason["parse_error"] != 1 || got.DropSetLoadError != "load failed" {
		t.Fatalf("round trip: %+v", got)
	}
	if got.DropSetLastFailure == nil || got.DropSetLastFailure.Reason != "parse_error" || got.DropSetLastFailure.Time.UTC() != now {
		t.Fatalf("lastFailure: %+v", got.DropSetLastFailure)
	}

	old := RuntimeInfo{HeadSeries: 3, DropSetFailOpen: 4}
	ob, _ := json.Marshal(old)
	var fromOld RuntimeInfo
	if err := json.Unmarshal(ob, &fromOld); err != nil {
		t.Fatal(err)
	}
	if fromOld.DropSetFailOpenByReason != nil || fromOld.DropSetLastFailure != nil {
		t.Fatal("old sidecar JSON must leave new fields unset, not zero-filled as healthy")
	}
}
