package dbsync

import (
	"slices"
	"sync"

	"github.com/andreaskoch/go-fswatch"
	"github.com/scarlass/tera-askep/internal/core/configs"
)

type SyncWatcher struct {
	watchersMu *sync.Mutex
	watchers   map[string]*FileWatcher
}

func NewWatcher() *SyncWatcher {
	watcher := &SyncWatcher{
		watchersMu: &sync.Mutex{},
		watchers:   make(map[string]*FileWatcher),
	}
	return watcher
}

func (sw *SyncWatcher) AddTarget(target configs.TargetConfig) *SyncWatcher {
	w := newFileWatcher(target)

	sw.watchersMu.Lock()
	sw.watchers[target.Name] = w
	sw.watchersMu.Unlock()
	return sw
}

func (sw *SyncWatcher) Start() error {
	return nil
}
func (sw *SyncWatcher) Stop() error {
	return nil
}

// ========================================================================================
// ========================================================================================

type FileWatcher struct {
	started  bool
	target   configs.TargetConfig
	watchers []*fswatch.FileWatcher
}

func newFileWatcher(target configs.TargetConfig) *FileWatcher {
	return &FileWatcher{
		target:   target,
		watchers: make([]*fswatch.FileWatcher, 0),
	}
}

type FileEvent struct {
	Target configs.TargetConfig
	File   string
}

func (fw *FileWatcher) Start(modified chan<- *FileEvent) {
	if fw.started {
		return
	}

	fw.started = true

	files := make([]string, 0)
	files = append(files, fw.target.Html)
	files = slices.Concat(files, fw.target.Script)
	files = slices.Concat(files, fw.target.Stylesheet)

	for _, file := range files {
		w := fswatch.NewFileWatcher(file, 2)
		fw.watchers = append(fw.watchers, w)

		w.Start()
		go func(w *fswatch.FileWatcher) {
			for w.IsRunning() {
				select {
				case <-w.Modified():
					modified <- &FileEvent{
						Target: fw.target,
						File:   file,
					}
				case <-w.Stopped():
					return
				case <-w.Moved():
					w.Stop()
					fw.watchers = slices.DeleteFunc(fw.watchers, func(sfw *fswatch.FileWatcher) bool { return sfw == w })
					return
				}
			}
		}(w)
	}
}
