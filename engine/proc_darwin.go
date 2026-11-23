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

// isNetworkFS: 네트워크 디스크(SMB · NFS · AFP · WebDAV)인지. Mac 전체 검색에서는 너무 느려서 빼요 ('선택한 폴더'로는 찾을 수 있어요)
func isNetworkFS(p string) bool {
	var st syscall.Statfs_t
	if syscall.Statfs(p, &st) != nil {
		return false
	}
	b := make([]byte, 0, len(st.Fstypename))
	for _, c := range st.Fstypename {
		if c == 0 {
			break
		}
		b = append(b, byte(c))
	}
	switch string(b) {
	case "smbfs", "nfs", "afpfs", "webdav", "ftp", "cifs", "cd9660":
		return true
	}
	return false
}
