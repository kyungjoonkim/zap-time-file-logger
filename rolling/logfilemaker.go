package rolling

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type FileTimeLogger struct {
	PrefixFileName string
	TimeFormat     string
	FilePeriod     time.Duration

	mu              sync.Mutex
	file            *os.File
	currentFullName string
	dir             string
	currentName     string
	currentTime     time.Time
}

const Ext = ".log"

// timeFormat 시간 포맷터 반환
func (f *FileTimeLogger) timeFormat() string {
	if f.TimeFormat == "" {
		f.TimeFormat = time.DateOnly
	}
	return f.TimeFormat
}

func (f *FileTimeLogger) prefixFimeName() string {
	if f.PrefixFileName == "" {
		f.PrefixFileName = "time-log-"
	}
	return f.PrefixFileName
}

// toBeFileName 파일명 생성
func (f *FileTimeLogger) toBeFileName() string {
	return makeFileName(f.prefixFimeName(), time.Now(), f.timeFormat())
}

func makeFileName(prefix string, targetTime time.Time, timeFormat string) string {
	return prefix + targetTime.Format(timeFormat) + Ext
}

// currentFileName 현재 파일명 반환
func (f *FileTimeLogger) currentFileName() string {
	return f.currentFullName
}

// updateFileName 파일명 갱신
func (f *FileTimeLogger) updateFileName(tobeName string) string {
	f.currentFullName = tobeName
	f.dir, f.currentName, f.currentTime = f.parseFileName(tobeName)
	return f.currentFullName
}

// openFile 파일 오픈
func (f *FileTimeLogger) openFile(fileName string) error {
	var err error
	if f.file != nil {
		if f.file.Name() == fileName {
			return nil
		}

		if closeErr := f.file.Close(); closeErr != nil {
			return closeErr
		}
	}

	if err = os.MkdirAll(f.dir, 0755); err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	f.file, err = os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	return err
}

func (f *FileTimeLogger) removeOldFiles() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in removeOldFiles", r)
		}
	}()

	if f.FilePeriod == 0 || f.currentTime.IsZero() {
		return
	}

	checkTime := f.currentTime.Add(-f.FilePeriod)
	dir := f.dir + string(filepath.Separator)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || path == f.currentFullName || !strings.HasPrefix(path, f.prefixFimeName()) {
			return nil
		}

		_, _, tTime := f.parseFileName(path)
		if tTime.IsZero() {
			return nil
		}

		if tTime.Before(checkTime) {
			err = os.Remove(path)
			if err != nil {
				fmt.Printf("Error removing old log file %v: %v\n", path, err)

			}
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error walking the path %v: %v\n", dir, err)
	}
	return
}

// Write 파일 쓰기
func (f *FileTimeLogger) Write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	curFileName := f.currentFileName()
	toBeFileName := f.toBeFileName()

	if curFileName != toBeFileName {
		curFileName = f.updateFileName(toBeFileName)
		go f.removeOldFiles()
	}

	if err = f.openFile(curFileName); err != nil {
		return 0, err
	}

	n, err = f.file.Write(p)
	return n, err
}

// Close implements io.Closer, and closes the current logfile.
func (f *FileTimeLogger) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.close()
}

// close closes the file if it is open.
func (f *FileTimeLogger) close() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

func (f *FileTimeLogger) parseFileName(totalFileName string) (string, string, time.Time) {
	path := filepath.Dir(totalFileName)
	fileName := filepath.Base(totalFileName)
	var targetTime time.Time

	if timeFormat, found := strings.CutPrefix(totalFileName, f.prefixFimeName()); found {
		if timeFormat, found = strings.CutSuffix(timeFormat, Ext); found {
			if parsTime, err := time.Parse(f.timeFormat(), timeFormat); err == nil {
				targetTime = parsTime
			}
		}
	}

	return path, fileName, targetTime
}
