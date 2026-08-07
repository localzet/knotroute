//go:build !windows

package certauth

import "errors"

func InstallUserRoot(a *Authority) error {
	return errors.New("automatic root installation is currently implemented on Windows; import root-ca.pem using your OS trust settings")
}
func UninstallUserRoot(a *Authority) error {
	return errors.New("automatic root removal is currently implemented on Windows")
}
