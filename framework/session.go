package framework

import (
	"sync"
	"time"
)

var globalSession map[string]Session

var globalSessionLock sync.RWMutex

var globalSessionExpireSeconds = 6000.00

type Session struct {
	sync.RWMutex
	id            string
	data          map[string]interface{}
	lastTouchTime time.Time
}

func newSession(context Context) {

}

func getSession(id string) Session {
	globalSessionLock.RLock()
	defer globalSessionLock.RUnlock()
	return globalSession[id]
}

func (this *Session) Set(key string, val interface{}) {
	this.Lock()
	defer this.Unlock()
	this.data[key] = val
}

func (this *Session) Get(key string) interface{} {
	this.RLock()
	defer this.RUnlock()
	return this.data[key]
}

func (this *Session) Id() string {
	return this.id
}

func init() {
	//session过期
	//Schedule("session-expire", 30*60, func() {
	//	globalSessionLock.Lock()
	//	defer globalSessionLock.Unlock()
	//	for k, v := range globalSession {
	//		v.Lock()
	//		if time.Now().Sub(v.lastTouchTime).Seconds() > globalSessionExpireSeconds {
	//			delete(globalSession, k)
	//		}
	//		v.Unlock()
	//	}
	//})
}
