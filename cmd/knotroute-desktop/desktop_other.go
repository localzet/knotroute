//go:build !windows

package main

import "fmt"

func runDesktop() {
	fmt.Println("knotroute-desktop is available on Windows; use knotroute run on this platform")
}
