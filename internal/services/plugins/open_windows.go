package plugins

import "golang.org/x/sys/windows"

func openDirectory(path string) error {
	value, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, nil, value, nil, nil, windows.SW_SHOWNORMAL)
}
