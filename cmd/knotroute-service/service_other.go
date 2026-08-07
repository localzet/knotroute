//go:build !windows

package main

import "fmt"

func runService() { fmt.Println("knotroute-service is only available on Windows") }
