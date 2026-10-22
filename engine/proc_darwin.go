//go:build darwin

package main

import (
	"os"
	"syscall"
	"time"
)

// birthTime: 파일을 만든 날짜
func birthTime(st os.FileInfo) time.Time {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return time.Unix(s.Birthtimespec.Sec, s.Birthtimespec.Nsec)
	}
	return st.ModTime()
}
