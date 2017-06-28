// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// move Format from writer, case format not responsibility of writer

package log

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// RotateHandler writes messages by lines limit, file size limit, or time frequency.
type RotateHandler struct {
	mw *MuxWriter

	FilePath string
	MaxLines int
	curLines int

	// Rotate at size
	MaxSize int
	curSize int

	// Rotate daily
	MaxDays  int
	openDate int

	Rotatable bool
	startLock sync.Mutex
}

// an *os.File writer with locker.
type MuxWriter struct {
	sync.Mutex
	logFile *os.File
}

// write to os.File.
func (l *MuxWriter) Write(b []byte) (int, error) {
	l.Lock()
	defer l.Unlock()
	return l.logFile.Write(b)
}

// set os.File in writer.
func (l *MuxWriter) SetLogFile(fd *os.File) {
	if l.logFile != nil {
		l.logFile.Close()
	}
	l.logFile = fd
}

// create a FileLogWriter returning as LoggerInterface.
func NewDefaultHandler(fp string) *RotateHandler {
	w := &RotateHandler{
		FilePath:  fp,
		Rotatable: false,
	}
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	return w
}

func NewDailyRotateHandler(fp string, days int) *RotateHandler {
	w := &RotateHandler{
		FilePath:  fp,
		MaxDays:   days,
		Rotatable: true,
	}
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	return w
}

func NewLinesRotateHandler(fp string, lines int) *RotateHandler {
	w := &RotateHandler{
		FilePath:  fp,
		MaxLines:  lines,
		Rotatable: true,
	}
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	return w
}

func NewSizeRotateHandler(fp string, size int) *RotateHandler {
	w := &RotateHandler{
		FilePath:  fp,
		MaxSize:   size,
		Rotatable: true,
	}
	// use MuxWriter instead direct use os.File for lock write when rotate
	w.mw = new(MuxWriter)
	return w
}

// inherit io.Writer
func (w *RotateHandler) Write(data []byte) (int, error) {
	if Debug {
		fmt.Println(string(data))
	}
	length := len(data)
	w.doCheckRotate(length)
	_, err := w.mw.Write(data)
	return length, err
}

func (w *RotateHandler) Init() {
	if len(w.FilePath) == 0 {
		panic(errors.New("config must have filename"))
	}

	fd, err := w.createLogFile()
	if err != nil {
		panic(err)
	}
	w.mw.SetLogFile(fd)
	if err = w.initLogFile(); err != nil {
		panic(err)
	}
}

func (w *RotateHandler) doCheckRotate(size int) {
	w.startLock.Lock()
	defer w.startLock.Unlock()
	if w.Rotatable && ((w.MaxLines > 0 && w.curLines >= w.MaxLines) ||
		(w.MaxSize > 0 && w.curSize >= w.MaxSize) ||
		(time.Now().Day() != w.openDate)) {
		if err := w.DoRotate(); err != nil {
			fmt.Fprintf(os.Stderr, "FileLogWriter(%q): %s\n", w.FilePath, err)
			return
		}
	}
	w.curLines++
	w.curSize += size
}

func (w *RotateHandler) createLogFile() (*os.File, error) {
	os.MkdirAll(filepath.Dir(w.FilePath), 0755)
	return os.OpenFile(w.FilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
}

func (w *RotateHandler) initLogFile() error {
	fd := w.mw.logFile
	fInfo, err := fd.Stat()
	if err != nil {
		return fmt.Errorf("get stat: %s\n", err)
	}
	w.curSize = int(fInfo.Size())
	w.openDate = time.Now().Day()
	if fInfo.Size() > 0 {
		content, err := ioutil.ReadFile(w.FilePath)
		if err != nil {
			return err
		}
		w.curLines = len(strings.Split(string(content), "\n"))
	} else {
		w.curLines = 0
	}
	return nil
}

// DoRotate means it need to write file in new file.
// new file name like xx.log.2013-01-01.2
func (w *RotateHandler) DoRotate() error {
	_, err := os.Lstat(w.FilePath)
	if err == nil { // file exists
		// Find the next available number
		num := 1
		fname := ""
		for ; err == nil && num <= 999; num++ {
			fname = w.FilePath + fmt.Sprintf(".%s.%03d", time.Now().Format("2006-01-02"), num)
			_, err = os.Lstat(fname)
		}
		// return error if the last file checked still existed
		if err == nil {
			return fmt.Errorf("rotate: cannot find free log number to rename %s\n", w.FilePath)
		}

		// block Logger's io.Writer
		w.mw.Lock()
		defer w.mw.Unlock()

		fd := w.mw.logFile
		fd.Close()

		// close fd before rename
		// Rename the file to its newfound home
		if err = os.Rename(w.FilePath, fname); err != nil {
			return fmt.Errorf("Rotate: %s\n", err)
		}

		// re-start logger
		w.Init()

		go w.deleteOldLog()
	}

	return nil
}

func (w *RotateHandler) deleteOldLog() {
	dir := filepath.Dir(w.FilePath)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) (returnErr error) {
		defer func() {
			if r := recover(); r != nil {
				returnErr = fmt.Errorf("Unable to delete old log '%s', error: %+v", path, r)
			}
		}()

		if !info.IsDir() && info.ModTime().Unix() < (time.Now().Unix()-int64(60*60*24*w.MaxDays)) {
			if strings.HasPrefix(filepath.Base(path), filepath.Base(w.FilePath)) {
				os.Remove(path)
			}
		}
		return returnErr
	})
}

// destroy file logger, close file writer.
func (w *RotateHandler) Close() {
	w.mw.logFile.Close()
}

// flush file logger.
// there are no buffering messages in file logger in memory.
// flush file means sync file from disk.
func (w *RotateHandler) Flush() {
	w.mw.logFile.Sync()
}
