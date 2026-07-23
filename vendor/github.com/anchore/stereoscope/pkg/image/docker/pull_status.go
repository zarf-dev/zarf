package docker

import (
	"sync"

	"github.com/wagoodman/go-progress"
)

const (
	UnknownPhase PullPhase = iota
	WaitingPhase
	PullingFsPhase
	DownloadingPhase
	DownloadCompletePhase
	ExtractingPhase
	VerifyingChecksumPhase
	AlreadyExistsPhase
	PullCompletePhase
)

var phaseLookup = map[string]PullPhase{
	"Waiting":            WaitingPhase,
	"Pulling fs layer":   PullingFsPhase,
	"Downloading":        DownloadingPhase,
	"Download complete":  DownloadCompletePhase,
	"Extracting":         ExtractingPhase,
	"Verifying Checksum": VerifyingChecksumPhase,
	"Already exists":     AlreadyExistsPhase,
	"Pull complete":      PullCompletePhase,
}

type PullPhase int
type LayerID string

type pullEvent struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	Progress       string `json:"progress,omitempty"`
	ProgressDetail struct {
		Current int `json:"current"`
		Total   int `json:"total"`
	} `json:"progressDetail"`
}

type LayerState struct {
	Phase            PullPhase
	PhaseProgress    progress.Progressable
	DownloadProgress progress.Progressable
}

type PullStatus struct {
	phaseProgress    map[LayerID]*progress.Manual
	downloadProgress map[LayerID]*progress.Manual
	phase            map[LayerID]PullPhase
	layers           []LayerID
	lock             sync.Mutex
	complete         bool
}

func newPullStatus() *PullStatus {
	return &PullStatus{
		phaseProgress:    make(map[LayerID]*progress.Manual),
		downloadProgress: make(map[LayerID]*progress.Manual),
		phase:            make(map[LayerID]PullPhase),
	}
}

func (p *PullStatus) Complete() bool {
	return p.complete
}

func (p *PullStatus) Layers() []LayerID {
	p.lock.Lock()
	defer p.lock.Unlock()

	return append([]LayerID{}, p.layers...)
}

func (p *PullStatus) Current(layer LayerID) LayerState {
	p.lock.Lock()
	defer p.lock.Unlock()

	return LayerState{
		Phase:            p.phase[layer],
		PhaseProgress:    progress.Progressable(p.phaseProgress[layer]),
		DownloadProgress: progress.Progressable(p.downloadProgress[layer]),
	}
}

func (p *PullStatus) onEvent(event *pullEvent) {
	p.lock.Lock()
	defer p.lock.Unlock()

	layer := LayerID(event.ID)
	if layer == "" {
		return
	}

	if _, ok := p.phaseProgress[layer]; !ok {
		// ignore the first layer as it's the image id
		if p.layers == nil {
			p.layers = make([]LayerID, 0)
			return
		}

		// this is a new layer, initialize tracking info
		p.phaseProgress[layer] = &progress.Manual{}
		p.downloadProgress[layer] = &progress.Manual{}
		p.layers = append(p.layers, layer)
	}

	// capture latest event info
	currentPhase, ok := phaseLookup[event.Status]
	if !ok {
		currentPhase = UnknownPhase
	}

	p.phase[layer] = currentPhase
	phaseProgress := p.phaseProgress[layer]

	if currentPhase >= AlreadyExistsPhase {
		phaseProgress.SetCompleted()
	} else {
		phaseProgress.Set(int64(event.ProgressDetail.Current))
		phaseProgress.SetTotal(int64(event.ProgressDetail.Total))
	}

	if currentPhase == DownloadingPhase {
		dl := p.downloadProgress[layer]
		dl.Set(int64(event.ProgressDetail.Current))
		dl.SetTotal(int64(event.ProgressDetail.Total))
	} else if currentPhase >= DownloadCompletePhase {
		dl := p.downloadProgress[layer]
		dl.Set(dl.Size())
		dl.SetCompleted()
	}
}
