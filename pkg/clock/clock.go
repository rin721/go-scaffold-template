package clock

import "time"

// Clock 抽象当前时间，便于业务和测试复用。
type Clock interface {
	Now() time.Time
	Sleep(time.Duration)
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func (systemClock) Sleep(duration time.Duration) {
	time.Sleep(duration)
}

// System 返回系统时钟。
func System() Clock {
	return systemClock{}
}

// Fixed 返回固定时间的测试时钟。
func Fixed(now time.Time) Clock {
	return fixedClock{now: now}
}

type fixedClock struct {
	now time.Time
}

func (c fixedClock) Now() time.Time {
	return c.now
}

func (c fixedClock) Sleep(time.Duration) {}

// RFC3339Millis 使用毫秒精度输出稳定时间文本。
func RFC3339Millis(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}
