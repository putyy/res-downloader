//go:build windows

package system

import (
	"testing"

	"golang.org/x/sys/windows"
)

func TestLocalMachineRootStoreIsOpenedReadOnly(t *testing.T) {
	if localMachineRootStoreOpenFlags&windows.CERT_SYSTEM_STORE_LOCAL_MACHINE == 0 {
		t.Fatal("local machine certificate store location is not selected")
	}
	if localMachineRootStoreOpenFlags&windows.CERT_STORE_READONLY_FLAG == 0 {
		t.Fatal("certificate store is not opened read-only")
	}
	if localMachineRootStoreOpenFlags&windows.CERT_STORE_OPEN_EXISTING_FLAG == 0 {
		t.Fatal("certificate store is allowed to be created")
	}
}
