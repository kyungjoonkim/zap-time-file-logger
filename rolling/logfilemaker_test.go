package rolling

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"strings"
	"testing"
	"time"
)

var fileLogger = &FileTimeLogger{
	PrefixFileName: "/tmp/app-err-",
	TimeFormat:     "2006-01-02",
	FilePeriod:     24 * time.Hour,
}

func TestSomething(t *testing.T) {
	assert.True(t, true, "True is true!")
}

func TestFileName(t *testing.T) {
	assert.Equal(t, fileLogger.PrefixFileName+time.Now().Format(fileLogger.TimeFormat)+Ext,
		fileLogger.toBeFileName())
}

func TestMakeFile(t *testing.T) {
	file := zapcore.AddSync(&FileTimeLogger{
		PrefixFileName: "app-err-",
		TimeFormat:     time.TimeOnly,
	})
	core := zapcore.NewTee(
		zapcore.NewCore(zapcore.NewConsoleEncoder(zap.NewProductionEncoderConfig()), file, zap.InfoLevel),
	)
	logger := zap.New(core)
	defer logger.Sync()
	logger.Info("Hello from Zap!")
}

func TestTimeCheck(t *testing.T) {
	old := time.Now().Add(-fileLogger.FilePeriod)
	fmt.Println(old.Format(fileLogger.TimeFormat))

	//assert.Equal(t, time.DateOnly, fileLogger.timeFormat())
}

func TestCutPrefix(t *testing.T) {
	after, found := strings.CutPrefix("/tmp/app-err-2024-09-16.log", fileLogger.prefixFimeName())
	if !found {
		fmt.Println(after)
		return
	}
	after, found = strings.CutSuffix(after, Ext)
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
	dir, name, currentTime := fileLogger.parseFileName("/tmp/app-err-2024-09-16.log")
	fmt.Printf("path : %s, name : %s, time : %s\n", dir, name, currentTime.Format(time.DateOnly))

}

func TestLog(t *testing.T) {
	logger := createLogger()
	defer logger.Sync()
	sugar := logger.Sugar()

	processEndTime := time.Now().Add(10 * time.Millisecond)

	goroutineLimit := make(chan struct{}, 10) // 최대 100개의 고루틴만 동시에 실행
	for processEndTime.After(time.Now()) {

		goroutineLimit <- struct{}{} // 고루틴 실행 전 채널에 값 추가
		go func() {
			defer func() {
				<-goroutineLimit
			}() // 고루틴 종료 후 채널에서 값 제거
			sugar.Infof("Hello from Zap!: %s", time.Now().Format("2006-01-02 15:04:05"))
		}()
	}
}

func createLogger() *zap.Logger {
	stdout := zapcore.AddSync(os.Stdout)
	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	developmentCfg := zap.NewDevelopmentEncoderConfig()
	developmentCfg.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(developmentCfg)

	core := zapcore.NewTee(
		zapcore.NewCore(consoleEncoder, stdout, level),
		newCoreFile(),
	)

	return zap.New(core)
}

func newCoreFile() zapcore.Core {
	//file1, _ := os.OpenFile("example.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	file := zapcore.AddSync(&FileTimeLogger{
		PrefixFileName: "log/app-err-",
		TimeFormat:     "2006-01-02 15:04:05",
		FilePeriod:     10 * time.Second,
	})

	//file := zapcore.AddSync(&lumberjack.Logger{
	//	Filename:   "example.log",
	//	MaxSize:    1, // megabytes
	//	MaxBackups: 0,
	//})

	productionCfg := zap.NewProductionEncoderConfig()
	productionCfg.TimeKey = "timestamp"
	productionCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	//&lumberjack.Logger{}

	return zapcore.NewCore(zapcore.NewConsoleEncoder(productionCfg), file, zap.InfoLevel)
}
