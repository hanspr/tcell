// +build darwin freebsd netbsd openbsd dragonfly

// Copyright 2015 The TCell Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcell

import (
	"os/signal"
	"syscall"
	"unsafe"

	"github.com/zyedidia/poller"
)

type termiosPrivate syscall.Termios

func (t *tScreen) termioInit() error {
	var e error
	var newtios termiosPrivate
	var fd uintptr
	var tios uintptr
	var ioc uintptr
	t.tiosp = &termiosPrivate{}

	if t.in, e = poller.Open("/dev/tty", poller.O_RO); e != nil {
		goto failed
	}
	if t.out, e = poller.Open("/dev/tty", poller.O_WO); e != nil {
		goto failed
	}

	tios = uintptr(unsafe.Pointer(t.tiosp))
	ioc = uintptr(syscall.TIOCGETA)
	fd = uintptr(t.out.(*poller.FD).Sysfd())
	if _, _, e1 := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioc, tios, 0, 0, 0); e1 != 0 {
		e = e1
		goto failed
	}

	// On this platform (FreeBSD and family), the baud rate is stored
	// directly as an integer in termios.c_ospeed.  No bitmasking required.
	t.baud = int(t.tiosp.Ospeed)
	newtios = *t.tiosp
	newtios.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK |
		syscall.ISTRIP | syscall.INLCR | syscall.IGNCR |
		syscall.ICRNL | syscall.IXON
	newtios.Oflag &^= syscall.OPOST
	newtios.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON |
		syscall.ISIG | syscall.IEXTEN
	newtios.Cflag &^= syscall.CSIZE | syscall.PARENB
	newtios.Cflag |= syscall.CS8

	// We wake up only when at least 1 byte has arrived
	newtios.Cc[syscall.VMIN] = 1
	newtios.Cc[syscall.VTIME] = 0
	tios = uintptr(unsafe.Pointer(&newtios))

	ioc = uintptr(syscall.TIOCSETA)
	if _, _, e1 := syscall.Syscall6(syscall.SYS_IOCTL, fd, ioc, tios, 0, 0, 0); e1 != 0 {
		e = e1
		goto failed
	}

	signal.Notify(t.sigwinch, syscall.SIGWINCH)

	if w, h, e := t.getWinSize(); e == nil && w != 0 && h != 0 {
		t.cells.Resize(w, h)
	}

	return nil

failed:
	if t.in != nil {
		t.in.(*poller.FD).Close()
	}
	if t.out != nil {
		t.out.(*poller.FD).Close()
	}
	return e
}

func (t *tScreen) termioFini() {

	signal.Stop(t.sigwinch)

	<-t.indoneq

	if t.out != nil {
		fd := uintptr(t.out.(*poller.FD).Sysfd())
		ioc := uintptr(syscall.TIOCSETAF)
		tios := uintptr(unsafe.Pointer(t.tiosp))
		syscall.Syscall6(syscall.SYS_IOCTL, fd, ioc, tios, 0, 0, 0)
		t.out.(*poller.FD).Close()
	}
	if t.in != nil {
		t.in.(*poller.FD).Close()
	}
}

func (t *tScreen) getWinSize() (int, int, error) {

	fd := uintptr(t.out.(*poller.FD).Sysfd())
	dim := [4]uint16{}
	dimp := uintptr(unsafe.Pointer(&dim))
	ioc := uintptr(syscall.TIOCGWINSZ)
	if _, _, err := syscall.Syscall6(syscall.SYS_IOCTL,
		fd, ioc, dimp, 0, 0, 0); err != 0 {
		return -1, -1, err
	}
	return int(dim[1]), int(dim[0]), nil
}
