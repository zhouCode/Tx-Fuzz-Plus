package runner

import "sync"

type InFlightSnapshot struct {
	Global            int            `json:"global"`
	PerLane           map[string]int `json:"per_lane,omitempty"`
	MaxGlobalObserved int            `json:"max_global_observed"`
	MaxLaneObserved   map[string]int `json:"max_lane_observed,omitempty"`
}

type InFlightLimiter struct {
	mu sync.Mutex

	globalCap int
	laneCap   int

	global  int
	perLane map[string]int

	maxGlobalObserved int
	maxLaneObserved   map[string]int
}

func NewInFlightLimiter(globalCap, laneCap int) *InFlightLimiter {
	return &InFlightLimiter{
		globalCap:       globalCap,
		laneCap:         laneCap,
		perLane:         make(map[string]int),
		maxLaneObserved: make(map[string]int),
	}
}

func (l *InFlightLimiter) TryAcquire(laneID string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.globalCap > 0 && l.global >= l.globalCap {
		return false
	}
	if l.laneCap > 0 && l.perLane[laneID] >= l.laneCap {
		return false
	}

	l.global++
	l.perLane[laneID]++
	if l.global > l.maxGlobalObserved {
		l.maxGlobalObserved = l.global
	}
	if l.perLane[laneID] > l.maxLaneObserved[laneID] {
		l.maxLaneObserved[laneID] = l.perLane[laneID]
	}
	return true
}

func (l *InFlightLimiter) Release(laneID string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.perLane[laneID] <= 0 || l.global <= 0 {
		return
	}
	l.global--
	l.perLane[laneID]--
	if l.perLane[laneID] == 0 {
		delete(l.perLane, laneID)
	}
}

func (l *InFlightLimiter) Snapshot() InFlightSnapshot {
	l.mu.Lock()
	defer l.mu.Unlock()

	perLane := make(map[string]int, len(l.perLane))
	for laneID, count := range l.perLane {
		perLane[laneID] = count
	}
	maxLaneObserved := make(map[string]int, len(l.maxLaneObserved))
	for laneID, count := range l.maxLaneObserved {
		maxLaneObserved[laneID] = count
	}

	return InFlightSnapshot{
		Global:            l.global,
		PerLane:           perLane,
		MaxGlobalObserved: l.maxGlobalObserved,
		MaxLaneObserved:   maxLaneObserved,
	}
}
