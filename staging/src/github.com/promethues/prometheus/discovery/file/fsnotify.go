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
	watchingPathMap map[string]map[string]bool
}

func NewWatcher() *Watcher {
	watcher, _ := fsnotify.NewWatcher()
	w := &Watcher{
		mutex:           sync.Mutex{},
		watcher:         watcher,
		chanMap:         make(map[string]chan fsnotify.Event),
		watchingPathMap: make(map[string]map[string]bool),
	}
	go w.watching()
	return w
}

func (w *Watcher) watching() {
	for {
		select {
		case event := <-w.watcher.Events:
			fmt.Printf("FileWatcher: try to get lock when sending event \n")
			w.mutex.Lock()
			fmt.Printf("FileWatcher: sending event get lock\n")
			for uuid, ch := range w.chanMap {
				fmt.Printf("FileWatcher: send event to chan %s \n", uuid)
				ch <- event
			}
			w.mutex.Unlock()
			fmt.Printf("FileWatcher: release event get lock\n")
		case err := <-w.watcher.Errors:
			if err != nil {
				fmt.Printf("%s msg=%s err=%s \n", time.Now().String(), "Error watching file", err.Error())
			}
		}
	}
}

func (w *Watcher) Register(uuid string, eventCh chan fsnotify.Event) {
	fmt.Printf("FileWatcher: Register chan %s \n", uuid)
	w.mutex.Lock()
	fmt.Printf("FileWatcher: Register try to get lock %s \n", uuid)
	fmt.Printf("FileWatcher: Register get lock %s \n", uuid)
	defer func() {
		w.mutex.Unlock()
		fmt.Printf("FileWatcher: Register release lock %s \n", uuid)
	}()
	w.chanMap[uuid] = eventCh
}

func (w *Watcher) UnRegister(uuid string) {
	fmt.Printf("FileWatcher: UnRegister path %s \n", uuid)
	fmt.Printf("FileWatcher: UnRegister try to get lock %s \n", uuid)
	w.mutex.Lock()
	fmt.Printf("FileWatcher: UnRegister get lock %s \n", uuid)
	defer func() {
		w.mutex.Unlock()
		fmt.Printf("FileWatcher: UnRegister release lock %s \n", uuid)
	}()
	delete(w.chanMap, uuid)
}

func (w *Watcher) AddPath(uuid, path string) error {
	fmt.Printf("FileWatcher: add path %s \n", path)
	fmt.Printf("FileWatcher: AddPath try to get lock %s %s \n", uuid, path)
	w.mutex.Lock()
	fmt.Printf("FileWatcher: AddPath get lock %s %s \n", uuid, path)
	defer func() {
		w.mutex.Unlock()
		fmt.Printf("FileWatcher: AddPath release lock %s %s \n", uuid, path)
	}()

	err := w.watcher.Add(path)
	if err != nil {
		return err
	}
	if _, ok := w.watchingPathMap[path]; !ok {
		w.watchingPathMap[path] = make(map[string]bool)
	}
	w.watchingPathMap[path][uuid] = true
	fmt.Printf("FileWatcher: %s watching count %d \n", path, len(w.watchingPathMap[path]))
	return nil
}

func (w *Watcher) RemovePath(uuid, path string) error {
	fmt.Printf("FileWatcher: removing path %s \n", path)
	fmt.Printf("FileWatcher: RemovePath try to get lock %s %s \n", uuid, path)
	w.mutex.Lock()
	fmt.Printf("FileWatcher: RemovePath get lock %s %s \n", uuid, path)
	defer func() {
		w.mutex.Unlock()
		fmt.Printf("FileWatcher: RemovePath release lock %s %s \n", uuid, path)
	}()

	delete(w.watchingPathMap[path], uuid)

	if len(w.watchingPathMap[path]) == 0 {
		fmt.Printf("FileWatcher: need to remove from fsWatcher, path %s \n", path)
		err := w.watcher.Remove(path)
		if err != nil {
			return err
		}
	}

	return nil
}
