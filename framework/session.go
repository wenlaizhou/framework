package framework

import (
	"sync"
	"time"
	"net/http"
)

var globalSession = make(map[string]Session)

var globalSessionLock sync.RWMutex

var globalSessionExpireSeconds = 6000.00

type Session struct {
	sync.RWMutex
	id            string
	data          map[string]interface{}
	lastTouchTime time.Time
}

func newSession(context Context) Session {

	id := Guid()
	s := Session{
		id:            id,
		data:          make(map[string]interface{}),
		lastTouchTime: time.Now(),
	}
	context.SetCookie(&http.Cookie{
		Name:     "sessionId",
		Value:    id,
		HttpOnly: true,
	})
	globalSessionLock.Lock()
	globalSession[id] = s
	globalSessionLock.Unlock()
	return s
}

func getSession(context Context) Session {
	s, ok := globalSession[context.GetCookie("sessionId")]
	if ok {
		return s
	}
	return newSession(context)
}

func (this Session) Set(key string, val interface{}) {
	this.data[key] = val
}

func (this Session) Get(key string) interface{} {
	return this.data[key]
}

func (this Session) Id() string {
	return this.id
}

func init() {
	//session过期
	Schedule("session-expire", 30*60, func() {
		for k, v := range globalSession {
			v.Lock()
			if time.Now().Sub(v.lastTouchTime).Seconds() > globalSessionExpireSeconds {
				delete(globalSession, k)
			}
			v.Unlock()
		}
	})
}
