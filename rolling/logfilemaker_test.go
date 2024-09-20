package rolling

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

var fileLogger = &TimeFileLogger{
	PrefixFileName:     "/tmp/app-err-",
	TimeFormat:         "2006-01-02",
	LogRetentionPeriod: 24 * time.Hour,
}

func TestSomething(t *testing.T) {

}

func TestTimeCheck(t *testing.T) {
	old := time.Now().Add(-fileLogger.LogRetentionPeriod)
	fmt.Println(old.Format(fileLogger.TimeFormat))

	//assert.Equal(t, time.DateOnly, fileLogger.timeFormat())
}

func TestCutPrefix(t *testing.T) {
	after, found := strings.CutPrefix("/tmp/app-err-2024-09-16.log", fileLogger.prefixFileName())
	if !found {
		fmt.Println(after)
		return
	}
	after, found = strings.CutSuffix(after, ext)
	if !found {
		fmt.Println(after)
		return
	}

	fmt.Println(after)
	nowFormat := time.Now().Format(fileLogger.timeFormat())
	fmt.Println(nowFormat)

	// 문자열을 time.Time 타입으로 변환
	parseTime, err := time.Parse(time.DateOnly, nowFormat)
	if err != nil {
		fmt.Println("Error parsing time:", err)
		return
	}

	fmt.Println("Parsed time:", parseTime)

}

func TestTimeDuration(t *testing.T) {
	logger := &TimeFileLogger{
		PrefixFileName:     "log/app-err-",
		TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
	}

	_ = logger.updateLogFileInfo(InitFile, time.Now())

	names := logger.removeFileNames()
	for _, name := range names {
		fmt.Println(name)
	}

}

func TestLog(t *testing.T) {

	processEndTime := time.Now().Add(30 * time.Second)

	goroutineLimit := make(chan struct{}, 10) // 최대 100개의 고루틴만 동시에 실행
	for processEndTime.After(time.Now()) {

		goroutineLimit <- struct{}{} // 고루틴 실행 전 채널에 값 추가
		go func() {
			defer func() {
				<-goroutineLimit
			}() // 고루틴 종료 후 채널에서 값 제거
		}()
	}
}
