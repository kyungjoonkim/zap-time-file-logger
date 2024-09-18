package rolling

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

type FileTimeLogger struct {
	PrefixFileName string
	TimeFormat     string
	FilePeriod     time.Duration
	MaxSize        int

	mu              sync.Mutex
	file            *os.File
	currentFullName string
	dir             string
	currentName     string
	fileIndex       int
	logFileTime     time.Time
}

// megabyte is the conversion factor between MaxSize and bytes.  It is a
// variable so tests can mock it out and not need to write megabytes of data
// to disk.
const (
	megabyte       = 1024 * 1024
	Ext            = ".log"
	defaultMaxSize = 100
	defaultName    = "time-log-"
	indexFormat    = "-%d"
)

var regex *regexp.Regexp

// Enum 타입 정의
type status int

// Enum 값 정의 (iota 사용)
const (
	InitFile status = iota
	NotChangeFile
	ChangeDateFile
	ChangeIndexFile
)

func init() {
	// 정규식 컴파일
	regex = regexp.MustCompile(`-\d+\.log$`)
}

// timeFormat 시간 포맷터 반환
func (f *FileTimeLogger) timeFormat() string {
	if f.TimeFormat == "" {
		f.TimeFormat = time.DateOnly
	}
	return f.TimeFormat
}

func (f *FileTimeLogger) prefixFimeName() string {
	if f.PrefixFileName == "" {
		f.PrefixFileName = defaultName
	}

	if f.PrefixFileName[len(f.PrefixFileName)-1:] != "-" {
		f.PrefixFileName += "-"
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

func (f *FileTimeLogger) removeOldFiles() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in removeOldFiles", r)
		}
	}()

	if f.FilePeriod == 0 || f.logFileTime.IsZero() {
		return
	}

	checkTime := f.logFileTime.Add(-f.FilePeriod)
	dir := f.dir + string(filepath.Separator)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || path == f.currentFullName || !strings.HasPrefix(path, f.prefixFimeName()) {
			return nil
		}

		tTime := f.parseFileName(path, checkTime.Location())
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
	return f.write(p)
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

func (f *FileTimeLogger) parseFileName(totalFileName string, location *time.Location) time.Time {
	var targetTime time.Time
	if trimmedFilename, found := strings.CutPrefix(totalFileName, f.prefixFimeName()); found {

		matches := regex.FindStringSubmatch(trimmedFilename)
		if len(matches) == 0 {
			return targetTime
		}

		timeFormat := regex.ReplaceAllString(trimmedFilename, "")
		if parsTime, err := time.ParseInLocation(f.timeFormat(), timeFormat, location); err == nil {
			return parsTime
		}
	}
	return targetTime
}

func (f *FileTimeLogger) write(p []byte) (n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	curTime := time.Now()

	logStatus, err := f.loggerFileStatus(curTime)
	if err != nil {
		return 0, err
	}

	if logStatus != NotChangeFile {
		err = f.updateFileInfo(curTime, logStatus)
		if err != nil {
			return 0, err
		}
		go f.removeOldFiles()
	}

	return f.file.Write(p)
}

// loggerFileStatus 파일 상태 체크
func (f *FileTimeLogger) loggerFileStatus(curTime time.Time) (status, error) {
	//init file
	if f.file == nil {
		return InitFile, nil
	}
	// check time change
	if f.logFileTime.Format(f.timeFormat()) != curTime.Format(f.timeFormat()) {
		return ChangeDateFile, nil
	}

	info, err := f.file.Stat()
	if err != nil {
		return 0, err
	}

	if info.Size() >= f.max() {
		return ChangeIndexFile, nil
	}

	return NotChangeFile, nil
}

func (f *FileTimeLogger) updateFileInfo(curTime time.Time, logStatus status) error {
	if logStatus == NotChangeFile {
		return nil
	}

	if logStatus == InitFile {
		f.dir = filepath.Dir(f.prefixFimeName())
		f.logFileTime = curTime
	}

	if logStatus == ChangeDateFile {
		f.logFileTime = curTime
		f.fileIndex = 0
	}

	if logStatus == ChangeIndexFile {
		f.fileIndex++
	}

	f.currentName = filepath.Base(f.prefixFimeName()) + f.logFileTime.Format(f.timeFormat()) + fmt.Sprintf(indexFormat, f.fileIndex) + Ext
	f.currentFullName = filepath.Join(f.dir, string(filepath.Separator), f.currentName)

	if err := f.close(); err != nil {
		return nil
	}

	if err := os.MkdirAll(f.dir, 0755); err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	var err error
	if f.file, err = os.OpenFile(f.currentFullName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644); err != nil {
		return err
	}

	return nil
}

// max returns the maximum size in bytes of log files before rolling.
func (f *FileTimeLogger) max() int64 {
	if f.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(f.MaxSize) * int64(megabyte)
}
