package adb_test

import (
	"log"

	adb "github.com/yosemite-open/go-adb"
)

func ExampleDevice_DoSyncLocalFile() {
	adbc, _ := adb.New()
	dev := adbc.Device(adb.AnyUsbDevice())

	sync, err := dev.DoSyncLocalFile("/data/local/tmp/tmp.txt", "local.txt", 0644)
	if err != nil {
		log.Fatal(err)
	}

Loop:
	for {
		select {
		case <-sync.C:
			log.Printf("transfered %v / %v bytes (%.2f%%)",
				sync.BytesCompleted(),
				sync.TotalSize,
				100*sync.Progress())
		case <-sync.DoneCopy:
			log.Printf("finish io copy")
		case <-sync.Done:
			log.Printf("finish system copy, this is final")
			break Loop
		}
	}
	log.Printf("copy error: %s", sync.Err())
}
