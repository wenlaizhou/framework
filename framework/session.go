package framework

import (
	"sync"
	"time"
)

var globalSession map[string]Session

var globalSessionLock sync.RWMutex

type Session struct {
	sync.RWMutex
	id            string
	data          map[string]interface{}
	lastTouchTime time.Time
}

func init() {
	//定期清除session
	Schedule("", 30*60, func() {
		globalSessionLock.Lock()
		defer globalSessionLock.Unlock()
	})
}
