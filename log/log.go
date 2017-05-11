package log

import (
	"log"
	"strings"
	"os"
	"path/filepath"
	"sync"
)

const (
	RotateModeNoRotate = iota
	RotateModeWeek
	RotateModeMonth
	RotateMode16M
	RotateMode256M
	RotateModeMillion
)

var Debug = false

type Vlogger struct {
	*log.Logger
	Name string
	FilePath string
	HandleMode int
}

func New(name, fp string, mode int)  *Vlogger{
	
	var handler *RotateHandler
	switch mode {
	case RotateModeNoRotate:
		handler = NewDefaultHandler(fp)
	case RotateModeWeek:
		handler = NewDailyRotateHandler(fp, 7)
	case RotateModeMonth:
		handler = NewDailyRotateHandler(fp, 30)
	case RotateMode16M:
		handler = NewSizeRotateHandler(fp, 1 << 24)
	case RotateMode256M:
		handler = NewSizeRotateHandler(fp, 1 << 28)
	case RotateModeMillion:
		handler = NewLinesRotateHandler(fp, 1000000)
	default:
		handler = NewDefaultHandler(fp)
	}
	handler.Init()
	logger := log.New(handler, strings.ToUpper(name)  +":", log.Ldate|log.Lmicroseconds)
	l :=  &Vlogger{
		Logger: logger,
		Name: name,
		HandleMode: mode,
	}
	
	return l
}

func (l *Vlogger) Error(v... interface{})  {
	l.Println(" >>>Error")
	l.Println(v)
}

type manager struct {
	mu     sync.Mutex
	baseDir string
	loggers map[string]*Vlogger
}

var bose = &manager{
	baseDir: "./",
	loggers: make(map[string]*Vlogger),
}

func SetLogDir(logDir string)  {
	if _, err := os.Stat(logDir); err != nil {
		log.Panicf("error when set log dir : %s", err)
	}
	bose.baseDir = logDir
}


func GetLogger(name string, mode int) *Vlogger {
	bose.mu.Lock()
	defer bose.mu.Unlock()
	
	if l , ok :=bose.loggers[name]; ok{
		return l
	}
	fp :=  filepath.Join(bose.baseDir, strings.ToLower(name)+".log")
	logger := New(name, fp, mode)
	bose.loggers[name] = logger
	return logger
}
