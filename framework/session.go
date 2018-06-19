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

func (this *Session) Set() {

}

func (this *Session) Get() {

}

func (this *Session) Id() {

}

func init() {
	//session过期
	Schedule("session-expire", 30*60, func() {
		globalSessionLock.Lock()
		defer globalSessionLock.Unlock()
		for k, v := range globalSession {
			v.Lock()
			if time.Now().Sub(v.lastTouchTime).Seconds() > globalSessionExpireSeconds {
				delete(globalSession, k)
			}
			v.Unlock()
		}
	})
}
