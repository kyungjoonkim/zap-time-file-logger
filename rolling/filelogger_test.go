package rolling

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestLog(t *testing.T) {

	processEndTime := time.Now().Add(1 * time.Second)

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

func TestLogFileName1(t *testing.T) {
	logger := &DateFileLogger{
		PrefixFileName:     "log/app-err",
		TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
		MaxSize:            1,
	}

	curTime := time.Now()
	if err := logger.checkInit(curTime); err != nil {

	}

	//wg := sync.WaitGroup{}
	logger.Write([]byte("test : " + time.Now().String() + "\n"))
	//wg.Add(1)
	//go func() {
	//	defer wg.Done()
	//	logger.saveBackupFile()
	//}()
	//wg.Wait()

	//logger.reNameOrRemoveOldFile(curTime)
}

func TestSaveNames(t *testing.T) {
	logger := &DateFileLogger{
		PrefixFileName:     "log/app-err",
		TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
		MaxSize:            1,
	}

	curTime := time.Now()
	if err := logger.checkInit(curTime); err != nil {

	}

	names := []string{
		//"app-err.log",
		//"app-err-1.log",
		//"app-err-2.log",
		"app-err-2024-09-16.log",
		"app-err-2024-09-16-1.log",
		//"app-err-2024-09-17.log",
		//"app-err-2024-09-18.log",
		//"app-err-2024-09-18-1.log",
		//"app-err-2024-09-18-2.log",
	}

	for _, name := range names {
		exist, val := logger.oldLogFileInfo(name, logger.curDir()+name, curTime.Location())
		fmt.Printf("exist : %t, val : %v\n", exist, val)
	}
}

func TestRenameCheck(t *testing.T) {
	logger := &DateFileLogger{
		PrefixFileName:     "log/app-err",
		TimeFormat:         "2006-01-02",
		LogRetentionPeriod: 24 * time.Hour,
		MaxSize:            1,
	}
	now := time.Now()
	logger.checkInit(now)
	logger.reNameOrRemoveOldFile(now)

}

func TestPaseMill(t *testing.T) {
	milliseconds, err := strconv.ParseInt("1727598347187", 10, 64)
	if err != nil {
		return
	}
	time2 := time.UnixMilli(milliseconds)
	fmt.Println(time2)
}

func TestCheckStr(t *testing.T) {
	logger := &DateFileLogger{
		PrefixFileName:     "log/app-err",
		TimeFormat:         "",
		LogRetentionPeriod: 24 * time.Hour,
		MaxSize:            1,
	}

	logger.fileInfo = &loggerFileInfo{
		timeFormat: "2006",
	}

}
