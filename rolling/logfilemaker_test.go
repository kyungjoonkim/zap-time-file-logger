package rolling

import (
	"fmt"
	"os"
	"path/filepath"
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
	timeFormat, existFormat := fileLogger.timeFormat()
	if !existFormat {
		return
	}

	nowFormat := time.Now().Format(timeFormat)
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

func TestTime(t *testing.T) {
	root, err := FindProjectRoot("go.mod")
	if err != nil {
		t.Fatal(err)
	}

	logPath := filepath.Join(root, "log")
	fmt.Println(logPath)
	testFileLogger := &TimeFileLogger{
		PrefixFileName: logPath + "/test-err",
		//TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
	}

	write, err := testFileLogger.Write([]byte("test : " + time.Now().String() + "\n"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(write)

}

func TestTimeFormat(t *testing.T) {
	ymdPattern := "204050504556767"
	timeStr := time.Now().Format(ymdPattern)

	tTime, err := time.Parse(ymdPattern, timeStr)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("time is zero : %t \n", tTime.IsZero())
}

func TestRoot(t *testing.T) {
	root, err := FindProjectRoot("go.mod")
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(root)
}

// FindProjectRoot searches for a project root directory by looking for a specific file or directory
// starting from the current directory and moving upwards.
func FindProjectRoot(markerFile string) (string, error) {
	// Start with the current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	// Keep going up the directory tree until we find the marker file/directory
	for {
		// Check if the marker file/directory exists in the current directory
		if _, err := os.Stat(filepath.Join(dir, markerFile)); err == nil {
			return dir, nil
		}

		// Get the parent directory
		parent := filepath.Dir(dir)

		// If we're already at the root directory, we've gone too far
		if parent == dir {
			return "", fmt.Errorf("could not find project root: no directory containing %s found", markerFile)
		}

		// Move up to the parent directory
		dir = parent
	}
}
