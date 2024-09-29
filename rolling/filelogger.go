package rolling

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DateFileLogger struct {
	PrefixFileName     string        // PrefixFileName is the prefix of the log file name. e.g. : tmp/app-err-
	TimeFormat         string        // TimeFormat is the time format for the log file name. e.g., 2006-01-02. Patterns defined in the time package should be used.
	LogRetentionPeriod time.Duration // LogRetentionPeriod is the duration of time to keep log files. e.g., 24 * time.Hour
	MaxSize            int           // MaxSize is the maximum size in megabytes of the log file before it gets rolled. Default is 100 megabytes.

	writeMutex sync.Mutex // writeMutex is used to lock the file for writing.
	fileInfo   *loggerFileInfo
	fileMutex  sync.Mutex
}

type paseFileInfo struct {
	totalFileName string
	fileName      string
	formatTime    string
	index         int
	tempTime      time.Time
}

func (logger *DateFileLogger) Write(logData []byte) (bytesWritten int, err error) {
	logger.writeMutex.Lock()
	defer logger.writeMutex.Unlock()

	curTime := time.Now()
	if err = logger.checkInit(curTime); err != nil {
		return 0, err
	}

	logFileStatus, err := logger.fileStatus(curTime, int64(len(logData)))
	if err != nil {
		return 0, err
	}

	if logFileStatus != NotChangeFile {
		err = logger.fileInfo.startFileChange(logFileStatus, curTime)
		if err != nil {
			return 0, err
		}

		go func() {
			logger.fileMutex.Lock()
			defer logger.fileMutex.Unlock()
			//todo: save backup file 작업 해야함
			logger.reNameOrRemoveOldFile(curTime)
		}()

	}

	return logger.fileInfo.file.Write(logData)
}

func (logger *DateFileLogger) checkInit(curTime time.Time) error {
	if logger.fileInfo != nil {
		return nil
	}
	return logger.makeLoggerInfo(curTime)
}

func (logger *DateFileLogger) fileStatus(curTime time.Time, logDataSize int64) (status, error) {
	fileInfo := logger.fileInfo

	if fileInfo.isChangeTime(curTime) {
		return ChangeDateFile, nil
	}

	fileSize, err := fileInfo.fileSize()
	if err != nil {
		return 0, err
	}

	if fileSize+logDataSize > logger.max() {
		return ChangeIndexFile, nil
	}

	return NotChangeFile, nil
}

// max returns the maximum size in bytes of log files before rolling.
func (logger *DateFileLogger) max() int64 {
	if logger.MaxSize == 0 {
		return int64(defaultMaxSize * megabyte)
	}
	return int64(logger.MaxSize) * int64(megabyte)
}

func (logger *DateFileLogger) makeLoggerInfo(curTime time.Time) error {
	prefixName := strings.TrimSpace(logger.PrefixFileName)
	dir := filepath.Dir(prefixName)
	filePrefix := filepath.Base(prefixName)
	format, formatedTime := makeValidDateFormat(logger.TimeFormat, curTime)
	fileName := makeFileName(filePrefix, formatedTime)
	fullFileName := makeFullFileName(dir, fileName)
	logFileTime := curTime

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("can't make directories for new logfile: %s", err)
	}

	file, err := openFile(fullFileName)
	if err != nil {
		return err
	}

	logger.fileInfo = &loggerFileInfo{
		dir:          dir,
		filePrefix:   filePrefix,
		timeFormat:   format,
		fileName:     fileName,
		fileIndex:    0,
		fullFileName: fullFileName,
		logFileTime:  logFileTime,
		file:         file,
	}
	return nil
}

func (logger *DateFileLogger) reNameOrRemoveOldFile(curTime time.Time) {

	backupFiles := make(map[string][]*paseFileInfo)
	err := filepath.WalkDir(logger.curDir(), func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() || logger.curFileName() == d.Name() {
			return nil
		}

		if isLogFile, fileInfo := logger.parseLogFile(d.Name(), path); isLogFile {
			parseFileInfos := backupFiles[fileInfo.fileName]
			parseFileInfos = append(parseFileInfos, fileInfo)
			backupFiles[fileInfo.fileName] = parseFileInfos
		}

		return nil
	})

	if err != nil {
		fmt.Printf("Error globbing the path %v: %v\n", logger.fileInfo.dir, err)
		return
	}

	for fileName, fileInfos := range backupFiles {
		if logger.canRemovalFile() {
			isRemove := false
			for _, info := range fileInfos {

				if logger.isRemoveFile(curTime, info.formatTime) {
					removeErr := os.Remove(info.totalFileName)
					if removeErr != nil {
						fmt.Printf("Error removing old log file %v: %v\n", info.totalFileName, removeErr)
					}
					isRemove = true
				}
			}
			if isRemove {
				break
			}
		}

		//fileInfos[0].
		indexMax := 0
		for _, info := range fileInfos {
			if info.index > indexMax {
				indexMax = info.index
			}
		}

		reNameFileInfos := make([]*paseFileInfo, 0)
		for _, info := range fileInfos {
			if info.tempTime.IsZero() {
				continue
			}
			reNameFileInfos = append(reNameFileInfos, info)
		}

		sort.Slice(reNameFileInfos, func(i, j int) bool {
			return reNameFileInfos[i].tempTime.Before(reNameFileInfos[j].tempTime)
		})

		for _, info := range reNameFileInfos {
			indexMax++
			targetFileName := logger.curDir() + fileName + hyphen + strconv.Itoa(indexMax) + ext
			err = os.Rename(info.totalFileName, targetFileName)
			for err != nil {
				indexMax++
				targetFileName = fileName + hyphen + strconv.Itoa(indexMax) + ext
				err = os.Rename(info.totalFileName, targetFileName)
			}
		}

	}

	fmt.Println(backupFiles)
}

func (logger *DateFileLogger) isRemoveFile(curTime time.Time, formatedTime string) bool {
	if !logger.canRemovalFile() || formatedTime == "" {
		return false
	}

	lastTime := curTime.Add(-logger.LogRetentionPeriod)
	extractTime, err := time.ParseInLocation(logger.fileInfo.timeFormat, formatedTime, curTime.Location())
	if err != nil {
		return false
	}
	return lastTime.After(extractTime)
}

func (logger *DateFileLogger) canRemovalFile() bool {
	return logger.LogRetentionPeriod > 0
}
func (logger *DateFileLogger) curDir() string {
	return logger.fileInfo.dir + string(filepath.Separator)
}

func (logger *DateFileLogger) curFileName() string {
	return logger.fileInfo.fileName
}

func (logger *DateFileLogger) parseLogFile(fileName string, path string) (bool, *paseFileInfo) {
	cutName, find := strings.CutPrefix(fileName, logger.fileInfo.filePrefix)
	if !find {
		return false, nil
	}

	// 0 : timeFormat-index,1 : ext
	splitFormatNames := strings.Split(cutName, ext)
	if len(splitFormatNames) < 2 {
		return false, nil
	}

	strMillTime := splitFormatNames[1]
	var tempTime time.Time
	if strMillTime != "" {
		strMillTime = strings.TrimPrefix(strMillTime, dot)
		milliseconds, err := strconv.ParseInt(strMillTime, 10, 64)
		if err != nil {
			return false, nil
		}
		tempTime = time.UnixMilli(milliseconds)
	}

	if len(logger.fileInfo.timeFormat) == 0 {
		indexVal := splitFormatNames[0]
		if len(indexVal) == 0 {
			return true, &paseFileInfo{
				totalFileName: path,
				fileName:      logger.fileInfo.filePrefix,
				tempTime:      tempTime,
			}
		}

		indexes := strings.Split(indexVal, hyphen)
		if len(indexes) != 2 {
			return false, nil
		}

		index, err := strconv.ParseInt(indexes[1], 10, 32)
		if err != nil {
			return false, nil
		}
		return true, &paseFileInfo{
			totalFileName: path,
			fileName:      logger.fileInfo.filePrefix,
			index:         int(index),
			tempTime:      tempTime,
		}
	} else {
		if splitFormatNames[0] == "" || len(splitFormatNames[0]) < 1 {
			return false, nil
		}

		timeAndIndex := splitFormatNames[0][1:]
		formatHyphenCnt := strings.Split(logger.fileInfo.timeFormat, hyphen)
		targetNameHyphenCnt := strings.Split(timeAndIndex, hyphen)

		if len(formatHyphenCnt) == len(targetNameHyphenCnt) {
			_, err := time.Parse(logger.fileInfo.timeFormat, timeAndIndex)
			if err != nil {
				return false, nil
			}

			return true, &paseFileInfo{
				totalFileName: path,
				fileName:      logger.fileInfo.filePrefix + hyphen + timeAndIndex,
				formatTime:    timeAndIndex,
				tempTime:      tempTime,
			}
		} else if len(formatHyphenCnt) < len(targetNameHyphenCnt) {
			targetTime := strings.Join(targetNameHyphenCnt[:len(targetNameHyphenCnt)-1], hyphen)
			_, err := time.Parse(logger.fileInfo.timeFormat, targetTime)
			if err != nil {
				return false, nil
			}

			strIndex := strings.Join(targetNameHyphenCnt[len(targetNameHyphenCnt)-1:], "")
			index, err := strconv.ParseInt(strIndex, 10, 32)
			if err != nil {
				return false, nil
			}

			return true, &paseFileInfo{
				totalFileName: path,
				fileName:      logger.fileInfo.filePrefix + hyphen + timeAndIndex,
				formatTime:    targetTime,
				index:         int(index),
				tempTime:      tempTime,
			}
		} else {
			return false, nil
		}

	}
}

//func (logger *DateFileLogger) prefixAndTime(fileName string) string {
//	cutName, find := strings.CutPrefix(fileName, logger.fileInfo.filePrefix)
//	if !find {
//		return ""
//	}
//
//	suffixSplit := strings.Split(cutName, ext)
//	if len(suffixSplit) < 2 {
//		return ""
//	}
//
//	if logger.fileInfo.timeFormat == "" {
//
//	}
//
//	//dateSize := strings.Split(logger.fileInfo.timeFormat, hyphen)
//	//cutNameSize := strings.Split(cutName, hyphen)
//	//if dateSize == cutNameSize {
//	//
//	//}
//
//	//filePreFix := logger.fileInfo.filePrefix
//
//	return cutName
//}

//func (logger *DateFileLogger) tempLogFile(fileName string) bool {
//	noPrefixLogName, find := strings.CutPrefix(logger.fileInfo.filePrefix, fileName)
//	if !find {
//		return false
//	}
//
//	tempTime, findTemp := strings.CutPrefix(fileName, ext)
//	if !findTemp {
//		return false
//	}
//
//	logTime, find := strings.CutSuffix(noPrefixLogName, ext)
//	if !find {
//		return false
//	}
//
//	if len(logTime) > 0 || len(logger.fileInfo.timeFormat) > 0 {
//
//	}
//
//	return strings.CutPrefix(logger.fileInfo.filePrefix, fileName)
//}

//func (logger *DateFileLogger) processBackupFile(backupPrefix string) error {
//	logFiles, err := filepath.Glob(filepath.Join(backupPrefix + "-*" + ext))
//	if err != nil {
//		return err
//	}
//
//	fileCnt := len(logFiles)
//	if fileCnt <= 0 {
//		return nil
//	}
//
//	var lastInfo fs.FileInfo
//	for _, backupFileName := range logFiles {
//		info, err := os.Stat(backupFileName)
//		if err != nil {
//			continue
//		}
//
//		if lastInfo == nil {
//			lastInfo = info
//			continue
//		}
//
//		if info.ModTime().After(lastInfo.ModTime()) {
//			lastInfo = info
//		}
//	}
//
//	if lastInfo == nil {
//		return nil
//	}
//
//	name, found := strings.CutPrefix(lastInfo.Name(), backupPrefix)
//	if !found {
//		return nil
//	}
//
//	strings.
//
//}
