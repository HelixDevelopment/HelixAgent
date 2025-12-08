package plugins

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/superagent/superagent/internal/utils"
)

// Watcher monitors plugin directories for changes and triggers reloads
type Watcher struct {
	watcher  *fsnotify.Watcher
	paths    []string
	onChange func(path string)
	stopChan chan struct{}
}

func NewWatcher(paths []string, onChange func(path string)) (*Watcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		watcher:  watcher,
		paths:    paths,
		onChange: onChange,
		stopChan: make(chan struct{}),
	}

	// Watch directories
	for _, path := range paths {
		if err := watcher.Add(path); err != nil {
			watcher.Close()
			return nil, err
		}
	}

	return w, nil
}

func (w *Watcher) Start() {
	go w.watchLoop()
	utils.GetLogger().Info("Started plugin file watcher")
}

func (w *Watcher) Stop() {
	close(w.stopChan)
	w.watcher.Close()
	utils.GetLogger().Info("Stopped plugin file watcher")
}

func (w *Watcher) watchLoop() {
	debounce := make(map[string]*time.Timer)

	for {
		select {
		case <-w.stopChan:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}

			// Only watch for plugin files
			if filepath.Ext(event.Name) != ".so" {
				continue
			}

			// Debounce events
			if timer, exists := debounce[event.Name]; exists {
				timer.Stop()
			}

			debounce[event.Name] = time.AfterFunc(500*time.Millisecond, func() {
				delete(debounce, event.Name)
				w.handleEvent(event)
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			utils.GetLogger().Errorf("File watcher error: %v", err)
		}
	}
}

func (w *Watcher) handleEvent(event fsnotify.Event) {
	switch {
	case event.Has(fsnotify.Create), event.Has(fsnotify.Write):
		utils.GetLogger().Infof("Plugin file changed: %s", event.Name)
		if w.onChange != nil {
			w.onChange(event.Name)
		}
	case event.Has(fsnotify.Remove), event.Has(fsnotify.Rename):
		utils.GetLogger().Infof("Plugin file removed: %s", event.Name)
	}
}
