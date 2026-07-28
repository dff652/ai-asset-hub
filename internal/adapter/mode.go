package adapter

// Default file modes for staged install.
const (
	ModeFile       uint32 = 0644
	ModePrivate    uint32 = 0600
	ModeExecutable uint32 = 0755
)

// FileMode returns the effective permission bits (defaults to 0644).
func FileMode(mode uint32) uint32 {
	if mode == 0 {
		return ModeFile
	}
	return mode
}
