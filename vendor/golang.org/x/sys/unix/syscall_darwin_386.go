// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build 386,darwin

package unix

import (
	"syscall"
)

//sys   ptrace(request int, pid int, addr uintptr, data uintptr) (err error)

func setTimespec(sec, nsec int64) Timespec {
	return Timespec{Sec: int32(sec), Nsec: int32(nsec)}
}

func setTimeval(sec, usec int64) Timeval {
	return Timeval{Sec: int32(sec), Usec: int32(usec)}
}

//sysnb	gettimeofday(tp *Timeval) (sec int32, usec int32, err error)
func Gettimeofday(tv *Timeval) (err error) {
	// The tv passed to gettimeofday must be non-nil
	// but is otherwise unused. The answers come back
	// in the two registers.
	sec, usec, err := gettimeofday(tv)
	tv.Sec = int32(sec)
	tv.Usec = int32(usec)
	return err
}

func SetKevent(k *Kevent_t, fd, mode, flags int) {
	k.Ident = uint32(fd)
	k.Filter = int16(mode)
	k.Flags = uint16(flags)
}

func (iov *Iovec) SetLen(length int) {
	iov.Len = uint32(length)
}

func (msghdr *Msghdr) SetControllen(length int) {
	msghdr.Controllen = uint32(length)
}

func (cmsg *Cmsghdr) SetLen(length int) {
	cmsg.Len = uint32(length)
}

func Syscall9(num, a1, a2, a3, a4, a5, a6, a7, a8, a9 uintptr) (r1, r2 uintptr, err syscall.Errno)

// SYS___SYSCTL is used by syscall_bsd.go for all BSDs, but in modern versions
// of darwin/386 the syscall is called sysctl instead of __sysctl.
const SYS___SYSCTL = SYS_SYSCTL

//sys	Fstat(fd int, stat *Stat_t) (err error) = SYS_FSTAT64
//sys	Fstatat(fd int, path string, stat *Stat_t, flags int) (err error) = SYS_FSTATAT64
//sys	Fstatfs(fd int, stat *Statfs_t) (err error) = SYS_FSTATFS64
//sys	Getdirentries(fd int, buf []byte, basep *uintptr) (n int, err error) = SYS_GETDIRENTRIES64
//sys	getfsstat(buf unsafe.Pointer, size uintptr, flags int) (n int, err error) = SYS_GETFSSTAT64
//sys	Lstat(path string, stat *Stat_t) (err error) = SYS_LSTAT64
//sys	Stat(path string, stat *Stat_t) (err error) = SYS_STAT64
//sys	Statfs(path string, stat *Statfs_t) (err error) = SYS_STATFS64
