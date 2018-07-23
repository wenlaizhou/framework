package framework

import (
	"fmt"
	"container/list"
)

var logList = list.New()

type logRecord struct {
	message string
	level   int
}

func Info(mark string, records ...interface{}) {
	logList.PushBack(fmt.Sprintf(mark, records...))
}

func Error(mark string, records ...interface{}) {
	fmt.Sprintf(mark, records...)
}

func Debug(mark string, records ...interface{}) {
	fmt.Sprintf(mark, records...)
}
