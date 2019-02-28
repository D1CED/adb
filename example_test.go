// An app demonstrating most of the library's features.
package adb_test

import (
	"fmt"
	"time"

	"github.com/d1ced/adb"
)

func Example() {

	client, _ := adb.NewDefault()

	serverVersion, _ := client.Version()
	fmt.Println("Server version:", serverVersion)

	deviceInfo, _ := client.ListDevices()

	fmt.Println("Devices:")
	for _, device := range deviceInfo {
		fmt.Println(device)
	}

	fmt.Println("Watching for device state changes.")
	watcher, err := client.NewDeviceWatcher()
	if err != nil {
		panic(err)
	}

	go func() {
		<-time.After(20 * time.Second)
		watcher.Close()
	}()

	for event := range watcher.C() {
		fmt.Printf("\t[%s]%+v\n", time.Now(), event)
	}
	if err = watcher.Err(); err != nil {
		fmt.Println(err)
	}

	client.Kill()
}
