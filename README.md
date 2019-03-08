# A Golang library for interacting with the Android Debug Bridge (adb).

A note from the fork's autor:

*This is a side project of mine. Do not expect any support or replies to
feature or bug request.*

The reason for this fork was that I wasn't happy with the internals.
There were simply too many types and it seemd overcomplex.

If you are looking for a propper adb client library in Go, take a look
a the implementation provided by Google.

## DONE (mostly):
Asyncwriter rewritten
  - now counts based on bytes written
  - does not filestat (racy?)
removed useage of runtime.SetFinalizer
  - this is considered bad style
  - better use Close properly
flatten
  - remove unessecarry types
  - remove top-level sybols
  - inline functions
  - do things in-place (tetra to le int)
  - get rid of dependencies (rhttp!?)
  - simplify names
simplified error handling (maybe readd some)
  - why internal package?
  - no buissness in this
  - now uses pkg/errors
removed useage of return parameters
moved away from []byte/strings to io.Reader
  - use more io.Copy, io.ReaderTo, io.WriterFrom
removed adb/wire
  - abstraction was broken anyways
  - don't export interfaces
  - moved to net.Conn and io.Reader/Writer
'log' useage of device watcher removed (moved to fmt.Printf, missing timestamp)
reimplement command handling
  - what about `echo :$?` (this is actually fine)
  - look into documentation for a better way to get exit status
rework how device descriptor work and how device connects to the server
  - always use serial as query-prefix `host-serial: &lt serial-number &gt : request
  - better track connection state (when reuse, when drop)
socket read/writes
  - only use two reads per message, header and body
  - only allocate once
  - use io.ReadFull
  - investigate if write full neccessary
  - reduce allocations for message sends
change ForwartSpec to a simple string
implement os.FileInfo for dir_entries
move cmd/demo and cmd/raw-adb to example files
implement formatter for deviceInfo
think about features of device_extra (what to keep/remove?)

## TODO:
write more tests (table driven style)
track (potential) leakages
expose low-level adb interface

Notice About hierarchy flattening:
See: https://github.com/golang/go/wiki/CodeReviewComments#interfaces

Device (new)
- Server
  - path
  - address
- serial

Device (old)
- sever (interface)
  - realServer
    - ServerConfig
      - PathToADB
      - Host
      - Port
      - Dialer (interface)
        - tcpDialer
        - wire.Conn
          - Scanner (interface)
            - realScanner
              - SyncScanner (interface)
                - realSyncScanner
                - StatusReader
            - StatusReader
          - Sender (interface)
            - realSender
            - SyncSender (interface)
              - realSyncSender
      - filesystem
    - address
- DeviceDescriptor
  - deviceDescriptorType
  - serial
- deviceListFunc

## Status
Host:
  - version [x]
  - kill [x]
  - devices(-l) [x]
  - track-devices [ ] needs investigation
 
Device:
  - get-devpath [x]
  - get-state; check this beforehand, poll?
  - Forward:
      - foreward
      - norebind
      - killforward
      - list-forward
  - shell [x]
  - remount [ ] result type need investigation, use this before sync/push
  - dev [x]
  - tcp?
  - local?
  - framebuffer?
  - jdwp?
  - reverse?

Sync:
  - List [x]
  - Send [ ] change abstraction to accepting reader
  - Retrieve [x]
  - Stat [x]

Other:
  - host: emulator; don't implement this, not for clients
  - device: product N/A
  - device: get-serialno; uneccessary as this is the device ID