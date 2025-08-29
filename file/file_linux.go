package file

import (
	"errors"

	"golang.org/x/sys/unix"
)

func isErrAttrNotExist(err error) bool {
	return errors.Is(err, unix.ENODATA)
}

func renameIfNotExist(oldpath, newpath string) error {
	return unix.Renameat2(unix.AT_FDCWD, oldpath, unix.AT_FDCWD, newpath, unix.RENAME_NOREPLACE)
}
