# zap-time-file-logger


zap Logger를 사용 하다 날짜별 로그 파일 생성이 없는거 같아서 한번 만들어 보았습니다.
zap Logger를 라이브러리가 이이 받아져 있어야 하며 사용중이여 합니다. 
설치가 이미 완료되 었다고 가정하에 설명을 진행 합니다.

사용법은 매우 간단하며 아래와 같습니다.

```go
zapcore.AddSync(&rolling.TimeFileLogger{
    PrefixFileName:     "xxxx/you-log-file-name",
    TimeFormat:         "2006-01-02",
    LogRetentionPeriod: 10 * 24 * time.Hour,
    MaxSize:            300,
})
```

위와 같이 사용하면 `xxxx/you-log-file-name-2020-01-01-0.log` 와 같은 형태로 날짜별로 로그 파일이 생성 됩니다.

### PrefixFileName:
- 로그 파일의 경로와 로그 파일 이름의 접두사를 지정 합니다. (xxxx/you-log-file-name)

### TimeFormat:
- 로그 파일 이름에 날짜를 어떤 형식으로 표시 할지 지정 합니다. Time.time package에 정의된 시간 포맷을 사용 합니다.

### LogRetentionPeriod:
- 로그 파일의 삭제 주기 입니다. 10 * 24 * time.Hour (10일) 파일 생성 기준 10 이전 파일은 삭제 됩니다.

### MaxSize:
- 로그 파일의 최대 크기 입니다. 300MB 이상이 되면 새로운 파일을 생성 합니다.

