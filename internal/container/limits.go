package container

// ResourceLimits specifies resource boundaries for command execution.
type ResourceLimits struct {
	MemoryBytes int64 // Maximum RAM (e.g., 2GB = 2 * 1024 * 1024 * 1024)
	NanoCPUs    int64 // CPU quota in nanoseconds (e.g., 2000000000 = 2 cores)
	PidsLimit   int64 // Maximum processes (e.g., 512)
}

// DefaultLimits returns standard safety limits for code execution.
func DefaultLimits() ResourceLimits {
	return ResourceLimits{
		MemoryBytes: 2 * 1024 * 1024 * 1024, // 2GB
		NanoCPUs:    2000000000,             // 2 Cores
		PidsLimit:   512,
	}
}
