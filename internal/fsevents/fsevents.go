package fsevents

// Event says that something in the folder at Path changed.
type Event struct {
	Path  string
	Flags uint32
	ID    uint64
}

// Flags from FSEvents that change what an event means.
const (
	MustScanSubDirs = 0x00000001 // changes below Path were coalesced: list everything under it again
	UserDropped     = 0x00000002 // events were lost: the next scan must be complete
	KernelDropped   = 0x00000004
	IDsWrapped      = 0x00000008
	HistoryDone     = 0x00000010 // the replay of past events is over; what follows is live
	RootChanged     = 0x00000020
)

// Lost reports whether events were dropped, so incremental updates can no longer be trusted.
func Lost(flags uint32) bool { return flags&(UserDropped|KernelDropped|IDsWrapped|RootChanged) != 0 }
