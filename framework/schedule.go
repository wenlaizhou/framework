package framework

import "time"
import "log"

var schedule []map[string]func()

func Schedule(name string, timeSchedule int, fun func()) {

	go func(timeSchedule int, fun func()) {
		for {
			time.Sleep(time.Second * time.Duration(timeSchedule))
			log.Printf("执行定时任务:%v, 时间间隔:%v", name, timeSchedule)
			fun()
		}
	}(timeSchedule, fun)
}
