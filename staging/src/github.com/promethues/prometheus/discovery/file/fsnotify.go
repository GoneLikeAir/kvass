package file

import (
	"fmt"
	"gopkg.in/fsnotify/fsnotify.v1"
	"sync"
	"time"
)

type Watcher struct {
	mutex           sync.Mutex
	watcher         *fsnotify.Watcher
	chanMap         map[string]chan fsnotify.Event
	watchingPathMap map[string]int
}

func NewWatcher() *Watcher {
	watcher, _ := fsnotify.NewWatcher()
	w := &Watcher{
		mutex:           sync.Mutex{},
		watcher:         watcher,
		chanMap:         make(map[string]chan fsnotify.Event),
		watchingPathMap: make(map[string]int),
	}
	go w.watching()
	return w
}

func (w *Watcher) watching() {
	ticker := time.NewTicker(time.Second * 30)
	defer ticker.Stop()
	for {
		select {
		case event := <-w.watcher.Events:
			w.mutex.Lock()
			for _, ch := range w.chanMap {
				ch <- event
			}
			w.mutex.Unlock()
		case err := <-w.watcher.Errors:
			if err != nil {
				fmt.Printf("%s msg=%s err=%s", time.Now().String(), "Error watching file", err.Error())
			}
		}
	}
}

func (w *Watcher) Register(uuid string, eventCh chan fsnotify.Event) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.chanMap[uuid] = eventCh
}

func (w *Watcher) UnRegister(uuid string) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	delete(w.chanMap, uuid)
}

func (w *Watcher) AddPath(uuid, path string) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	err := w.watcher.Add(path)
	if err != nil {
		return err
	}
	if _, ok := w.watchingPathMap[path]; !ok {
		w.watchingPathMap[path] = 0
	}
	w.watchingPathMap[path] = w.watchingPathMap[path] + 1
	return nil
}

func (w *Watcher) RemovePath(uuid, path string) error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.watchingPathMap[path] > 0 {
		w.watchingPathMap[path] = w.watchingPathMap[path] - 1
	}
	if w.watchingPathMap[path] == 0 {
		err := w.watcher.Remove(path)
		if err != nil {
			return err
		}
	}

	return nil
}
