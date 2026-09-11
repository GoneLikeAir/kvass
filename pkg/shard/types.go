/*
 * Tencent is pleased to support the open source community by making TKEStack available.
 *
 * Copyright (C) 2012-2019 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */

package shard

import (
	"time"
	"tkestack.io/kvass/pkg/target"
)

// ReplicasManager known all shard managers
type ReplicasManager interface {
	// Replicas return all replicas
	Replicas() ([]Manager, error)
}

// Manager known how to create or delete Shards
type Manager interface {
	// Shards return current Shards in the cluster
	Shards() ([]*Shard, error)
	// ChangeScale create or delete Shards according to "expReplicate"
	ChangeScale(expReplicate int32) error
}

// RuntimeInfo contains all running status of this shard
type RuntimeInfo struct {
	// HeadSeries return current head_series of prometheus
	HeadSeries int64 `json:"headSeries"`
	// ConfigHash is the md5 of current config file
	ConfigHash string `json:"ConfigHash"`
	// IdleStartAt is the time that shard begin idle
	IdleStartAt *time.Time `json:"IdleStartAt,omitempty"`
	// DropSetHash is sha256 of the loaded idle-drop name list
	DropSetHash string `json:"dropSetHash,omitempty"`
	// DropSetEnabled is whether idle-drop filtering is active
	DropSetEnabled bool `json:"dropSetEnabled"`
	// DropSetGeneration is the loaded drop-set generation
	DropSetGeneration string `json:"dropSetGeneration,omitempty"`
	// DropSetFailOpen counts rewrite failures that forwarded the original body
	DropSetFailOpen uint64 `json:"dropSetFailOpen"`
	// DropSetFailOpenByReason is per-reason fail-open counts; sum equals DropSetFailOpen
	DropSetFailOpenByReason map[string]uint64 `json:"dropSetFailOpenByReason,omitempty"`
	// DropSetLoadError is the current drop-set load error, not covered by proxy history
	DropSetLoadError string `json:"dropSetLoadError,omitempty"`
	// DropSetLastError is the last drop-set load or rewrite error
	DropSetLastError string `json:"dropSetLastError,omitempty"`
	// DropSetLastFailure is the most recent fail-open event, not a current-fault flag
	DropSetLastFailure *DropSetLastFailure `json:"dropSetLastFailure,omitempty"`
}

// DropSetLastFailure is a historical fail-open event snapshot.
type DropSetLastFailure struct {
	Reason          string    `json:"reason"`
	Time            time.Time `json:"time"`
	Job             string    `json:"job"`
	TargetID        string    `json:"targetId"`
	Address         string    `json:"address,omitempty"`
	ContentType     string    `json:"contentType,omitempty"`
	FilteringActive bool      `json:"filteringActive"`
	Generation      string    `json:"generation,omitempty"`
}

// UpdateTargetsRequest contains all information about the targets updating request
type UpdateTargetsRequest struct {
	// targets contains all targets this shard should scrape
	Targets map[string][]*target.Target
}

// UpdateConfigRequest is request struct for POST /
type UpdateConfigRequest struct {
	RawContent string `json:"rawContent"`
}
