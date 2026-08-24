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
	// DropSetLastError is the last drop-set load or rewrite error
	DropSetLastError string `json:"dropSetLastError,omitempty"`
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
